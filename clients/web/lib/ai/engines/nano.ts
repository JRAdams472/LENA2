// Chrome's built-in model (Gemini Nano) via the Prompt API — the
// zero-download path: Chrome ships the model. Each chat call creates a
// session seeded with the conversation so far; sessions are tiny-context
// so the agent keeps turns short.

import { nanoHandle } from "../capabilities";
import type { EngineMessage, LocalEngine } from "../types";

export class NanoEngine implements LocalEngine {
  readonly id = "nano" as const;
  readonly label = "On this device";

  private constructor(private readonly handle: NonNullable<ReturnType<typeof nanoHandle>>) {}

  // create returns null when the Prompt API isn't usable right now —
  // availability is probed separately for UX, this is the last sanity gate.
  static async create(): Promise<NanoEngine | null> {
    const h = nanoHandle();
    if (!h) return null;
    try {
      // Smoke-test session creation eagerly so a broken origin trial or
      // storage-full state falls back before the first user turn.
      const s = await h.create();
      s.destroy?.();
      return new NanoEngine(h);
    } catch {
      return null;
    }
  }

  async chat(messages: EngineMessage[]): Promise<string> {
    const last = messages[messages.length - 1];
    const history = messages.slice(0, -1);

    let session;
    try {
      // Current Prompt API shape: initialPrompts carries system + turns.
      session = await this.handle.create({
        initialPrompts: history.map((m) => ({ role: m.role, content: m.content })),
      });
    } catch {
      // Origin-trial-era shape: systemPrompt option + no history seeding —
      // the agent then sends the full transcript as one user turn.
      session = await this.handle.create();
      const transcript = messages
        .map((m) => `${m.role === "system" ? "Instructions" : m.role}: ${m.content}`)
        .join("\n\n");
      const reply = await session.prompt(transcript);
      session.destroy?.();
      return reply;
    }

    try {
      if (last.role !== "user") {
        // Nothing new to prompt — replay the tail turn as a user message.
        return await session.prompt(`user: ${last.content}`);
      }
      return await session.prompt(last.content);
    } finally {
      session.destroy?.();
    }
  }

  destroy(): void {
    /* sessions are per-call; nothing to release */
  }
}
