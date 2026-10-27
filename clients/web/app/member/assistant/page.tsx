"use client";

import { useEffect, useRef, useState } from "react";
import NextLink from "next/link";
import { useMutation } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import CircularProgress from "@mui/material/CircularProgress";
import LinearProgress from "@mui/material/LinearProgress";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import AutoAwesomeIcon from "@mui/icons-material/AutoAwesome";
import { useAssistant } from "@/lib/ai/useAssistant";
import type { AssistantResult } from "@/lib/ai/types";

interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  text: string;
  tools?: string[];
}

// Quick prompts seed the chat; the capability chips link to surfaces that
// actually exist today — no promised-feature dead ends.
const QUICK_PROMPTS = [
  "What's expiring in my pantry soon?",
  "What should I cook this week?",
  "What's in my wine cellar?",
];

const CAPABILITIES = [
  { label: "Meal plans", href: "/member/meal-plans" },
  { label: "Recipes", href: "/member/recipes" },
  { label: "Wine cellar", href: "/member/wine/bottles" },
];

// Conversation survives in-app navigation for the browser session only —
// nothing is kept across sessions or shared with other tabs.
const CHAT_KEY = "lena-assistant-chat";

function loadMessages(): ChatMessage[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.sessionStorage.getItem(CHAT_KEY);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (m): m is ChatMessage =>
        typeof m === "object" &&
        m !== null &&
        typeof (m as ChatMessage).id === "string" &&
        ((m as ChatMessage).role === "user" ||
          (m as ChatMessage).role === "assistant") &&
        typeof (m as ChatMessage).text === "string"
    );
  } catch {
    return [];
  }
}

export default function AssistantPage() {
  const dot = useAssistant();
  const [messages, setMessages] = useState<ChatMessage[]>(loadMessages);
  const [input, setInput] = useState("");
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    try {
      if (messages.length === 0) {
        window.sessionStorage.removeItem(CHAT_KEY);
      } else {
        window.sessionStorage.setItem(CHAT_KEY, JSON.stringify(messages));
      }
    } catch {
      // sessionStorage full/unavailable — the chat still works in-memory.
    }
  }, [messages]);

  const clearChat = () => {
    setMessages([]);
    try {
      window.sessionStorage.removeItem(CHAT_KEY);
    } catch {
      // ignore — clearing state already dropped the visible history
    }
  };

  const ask = useMutation({
    mutationFn: (question: string) => dot.ask(question),
    onSuccess: (answer: AssistantResult) => {
      setMessages((prev) => [
        ...prev,
        { id: crypto.randomUUID(), role: "assistant", text: answer.answer, tools: answer.tools },
      ]);
      setError(null);
      inputRef.current?.focus();
    },
    onError: (e) => {
      setError(e instanceof Error ? e.message : "The assistant didn't answer");
    },
  });

  const send = (question: string) => {
    const q = question.trim();
    if (!q || ask.isPending) return;
    setMessages((prev) => [...prev, { id: crypto.randomUUID(), role: "user", text: q }]);
    setInput("");
    ask.mutate(q);
  };

  if (dot.status === null) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", mt: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (!dot.available) {
    return (
      <Paper sx={{ maxWidth: 560, p: 3 }}>
        <Typography variant="h5" gutterBottom>
          Ask Dot
        </Typography>
        <Alert severity="info">
          Dot can't run here — this browser can't run a local model and the
          server has no AI provider configured.
        </Alert>
      </Paper>
    );
  }

  return (
    <Paper sx={{ maxWidth: 720, p: 3 }}>
      <Stack direction="row" spacing={1} sx={{ mb: 1, alignItems: "center" }}>
        <AutoAwesomeIcon color="primary" />
        <Typography variant="h5">Ask Dot</Typography>
        <Chip
          size="small"
          variant="outlined"
          color={dot.localActive ? "primary" : "default"}
          label={dot.localActive ? "On this device" : "Via server"}
        />
        {messages.length > 0 && (
          <Button size="small" sx={{ ml: "auto" }} onClick={clearChat}>
            Clear conversation
          </Button>
        )}
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Ask about your pantry, meal plan, recipes, or cellar. Every answer shows
        which household lookups the assistant made.
      </Typography>

      {dot.status === "opt-in" && (
        <Alert
          severity="info"
          sx={{ mb: 2 }}
          action={
            <Button color="inherit" size="small" onClick={() => void dot.enableLocal()}>
              Enable
            </Button>
          }
        >
          This browser can run Dot locally — answers would be generated on your
          device instead of the server.
        </Alert>
      )}
      {dot.status === "downloading" && dot.download && (
        <Box sx={{ mb: 2 }}>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 0.5 }}>
            Downloading the on-device model — {Math.round(dot.download.fraction * 100)}%
          </Typography>
          <LinearProgress
            variant="determinate"
            value={Math.round(dot.download.fraction * 100)}
          />
        </Box>
      )}
      {dot.downloadError && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          Couldn't set up the local model ({dot.downloadError}) — Dot will keep
          using the server.
        </Alert>
      )}
      {dot.localActive && (
        <Typography variant="body2" sx={{ mb: 2 }}>
          <Button size="small" onClick={() => dot.setServerOnly(true)}>
            Use server instead
          </Button>
        </Typography>
      )}

      <Stack direction="row" spacing={1} sx={{ mb: 2, flexWrap: "wrap" }} useFlexGap>
        {CAPABILITIES.map((c) => (
          <Chip
            key={c.href + c.label}
            component={NextLink}
            href={c.href}
            label={c.label}
            variant="outlined"
            clickable
          />
        ))}
      </Stack>

      {messages.length === 0 && (
        <Stack direction="row" spacing={1} sx={{ mb: 2, flexWrap: "wrap" }} useFlexGap>
          {QUICK_PROMPTS.map((p) => (
            <Chip key={p} label={p} onClick={() => send(p)} />
          ))}
        </Stack>
      )}

      <Stack spacing={1.5} sx={{ mb: 2 }}>
        {messages.map((m) => (
          <Box
            key={m.id}
            sx={{
              alignSelf: m.role === "user" ? "flex-end" : "flex-start",
              maxWidth: "85%",
              bgcolor: m.role === "user" ? "primary.main" : "action.hover",
              color: m.role === "user" ? "primary.contrastText" : "text.primary",
              borderRadius: 3,
              px: 2,
              py: 1,
            }}
          >
            <Typography variant="body2" sx={{ whiteSpace: "pre-wrap" }}>
              {m.text}
            </Typography>
            {m.tools && m.tools.length > 0 && (
              <Typography variant="caption" sx={{ opacity: 0.75, display: "block", mt: 0.5 }}>
                looked up: {m.tools.join(", ")}
              </Typography>
            )}
          </Box>
        ))}
        {ask.isPending && (
          <Stack direction="row" spacing={1} sx={{ alignItems: "center" }}>
            <CircularProgress size={16} />
            <Typography variant="body2" color="text.secondary">
              Dot is thinking…
            </Typography>
          </Stack>
        )}
      </Stack>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      <Stack direction="row" spacing={1} component="form" onSubmit={(e) => { e.preventDefault(); send(input); }}>
        <TextField
          inputRef={inputRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Ask Dot…"
          size="small"
          fullWidth
          autoFocus
        />
        <Button type="submit" variant="contained" disabled={ask.isPending || !input.trim()}>
          Send
        </Button>
      </Stack>
    </Paper>
  );
}
