"use client";

import { useRef, useState } from "react";
import NextLink from "next/link";
import { useMutation, useQuery } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import CircularProgress from "@mui/material/CircularProgress";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import SmartToyIcon from "@mui/icons-material/SmartToy";
import { api, ApiError } from "@/lib/api";
import { AssistantAnswer } from "@/lib/types";

interface ChatMessage {
  role: "user" | "assistant";
  text: string;
  tools?: string[];
}

// Quick prompts seed the chat; the capability chips jump to the richer
// reviewable surfaces built in ai-p3..p5.
const QUICK_PROMPTS = [
  "What's expiring in my pantry soon?",
  "What should I cook this week?",
  "What's in my wine cellar?",
];

const CAPABILITIES = [
  { label: "Suggest meals", href: "/meal-plans" },
  { label: "Cocktail ideas", href: "/recipes" },
  { label: "Wine pairing", href: "/recipes" },
];

export default function AssistantPage() {
  const { data: available, isLoading } = useQuery({
    queryKey: ["aiAvailable"],
    queryFn: api.getAIAvailable,
  });
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const ask = useMutation({
    mutationFn: (question: string) => api.askAssistant(question),
    onSuccess: (answer: AssistantAnswer) => {
      setMessages((prev) => [
        ...prev,
        {
          role: "assistant",
          text: answer.answer,
          tools: answer.toolCalls.map((t) => t.name),
        },
      ]);
      setError(null);
      inputRef.current?.focus();
    },
    onError: (e) => {
      setError(e instanceof ApiError ? e.message : "The assistant didn't answer");
    },
  });

  const send = (question: string) => {
    const q = question.trim();
    if (!q || ask.isPending) return;
    setMessages((prev) => [...prev, { role: "user", text: q }]);
    setInput("");
    ask.mutate(q);
  };

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", mt: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (!available) {
    return (
      <Paper sx={{ maxWidth: 560, p: 3 }}>
        <Typography variant="h5" gutterBottom>
          Assistant
        </Typography>
        <Alert severity="info">
          LENA's assistant isn't configured on this server — no AI provider is
          set, so meal suggestions, event fixes, and pairing ideas are off too.
        </Alert>
      </Paper>
    );
  }

  return (
    <Paper sx={{ maxWidth: 720, p: 3 }}>
      <Stack direction="row" spacing={1} sx={{ mb: 1, alignItems: "center" }}>
        <SmartToyIcon color="primary" />
        <Typography variant="h5">Ask LENA</Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        Ask about your pantry, meal plan, recipes, or cellar. Every answer shows
        which household lookups the assistant made.
      </Typography>

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
        {messages.map((m, i) => (
          <Box
            key={i}
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
              LENA is thinking…
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
          placeholder="Ask LENA…"
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
