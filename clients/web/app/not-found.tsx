import Link from "next/link";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Typography from "@mui/material/Typography";

// Root 404 — replaces the unstyled framework default.
export default function NotFound() {
  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        minHeight: "60vh",
        gap: 2,
        px: 2,
        textAlign: "center",
      }}
    >
      <Typography variant="h2">Page not found</Typography>
      <Typography variant="body2" color="text.secondary" sx={{ maxWidth: 400 }}>
        The page you were looking for doesn&apos;t exist or may have been
        moved.
      </Typography>
      <Button variant="contained" component={Link} href="/">
        Back to Lena
      </Button>
    </Box>
  );
}
