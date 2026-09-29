import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";

interface LenaLogoProps {
  size?: number;
  iconColor?: string;
  textColor?: string;
  showWordmark?: boolean;
}

export default function LenaLogo({
  size = 28,
  iconColor = "#059669",
  textColor = "#1e293b",
  showWordmark = true,
}: LenaLogoProps) {
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
      <Box
        component="svg"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth={2.5}
        strokeLinecap="round"
        strokeLinejoin="round"
        sx={{ width: size, height: size, color: iconColor, flexShrink: 0 }}
      >
        <path d="M2 20h20" />
        <path d="M20 16A8 8 0 0 0 4 16v0" />
        <path d="M12 4v4" />
        <circle cx="12" cy="3" r="1" fill="currentColor" />
      </Box>
      {showWordmark && (
        <Typography
          component="span"
          sx={{
            fontWeight: 600,
            letterSpacing: "0.05em",
            fontSize: size * 0.72,
            color: textColor,
            lineHeight: 1,
          }}
        >
          LENA
        </Typography>
      )}
    </Box>
  );
}
