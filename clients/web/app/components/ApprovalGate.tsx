"use client";

import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Fade from "@mui/material/Fade";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import HourglassTopIcon from "@mui/icons-material/HourglassTop";
import BlockIcon from "@mui/icons-material/Block";
import { useAuth } from "@/app/auth/AuthProvider";
import LenaLogo from "@/app/components/LenaLogo";

// Full-screen gate for signed-in users the API rejects with a 403
// approval status — pending accounts see "access request sent",
// denied accounts see a blocking message. Rendered by AdminLayout in
// place of member chrome so no household data ever mounts.
export default function ApprovalGate({ denied = false }: { denied?: boolean }) {
  const { signOut } = useAuth();

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        minHeight: "100vh",
        px: 2,
      }}
    >
      <Fade in timeout={400}>
        <Paper
          elevation={3}
          sx={{
            maxWidth: 420,
            width: "100%",
            p: { xs: 4, sm: 5 },
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
            gap: 2,
            textAlign: "center",
          }}
        >
          <LenaLogo size={32} />
          {denied ? (
            <BlockIcon color="error" sx={{ fontSize: 40 }} />
          ) : (
            <HourglassTopIcon color="primary" sx={{ fontSize: 40 }} />
          )}
          <Typography variant="h6">
            {denied ? "Access not granted" : "Access request sent"}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {denied
              ? "An administrator has declined this account's request for access. If you believe this is a mistake, contact your household administrator."
              : "An administrator will review your request before you can use LENA. You will gain access once approved — check back soon."}
          </Typography>
          <Button variant="outlined" onClick={signOut}>
            Sign out
          </Button>
        </Paper>
      </Fade>
    </Box>
  );
}
