import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";

// Route-level suspense fallback — keeps the layout shell up while a
// page-level async boundary resolves instead of flashing a blank frame.
export default function Loading() {
  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        minHeight: "60vh",
      }}
    >
      <CircularProgress aria-label="Loading" />
    </Box>
  );
}
