import Typography from "@mui/material/Typography";

// Shown where the hidden alcohol-suggestion entry point would sit when
// the caller lacks a 21+ birthdate (missing or under-age) — store
// compliance asks for a clear reason rather than a silently absent button.
export default function AlcoholGateHint() {
  return (
    <Typography
      variant="caption"
      color="text.secondary"
      data-testid="alcohol-gate-hint"
    >
      Wine pairings and cocktail suggestions are restricted to users 21+.
      You can update your birthdate anytime in your Profile settings.
    </Typography>
  );
}
