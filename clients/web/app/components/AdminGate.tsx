"use client";

import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";
import { useMe } from "@/app/auth/useMe";

// Route-level admin check for catalog/admin pages — children only mount
// for admins, so member-role users never fire the page's data queries.
// Matches the "Forbidden: this page requires the admin role." copy used
// by the users and pending-review pages.
export default function AdminGate({
  children,
}: {
  children: React.ReactNode;
}) {
  const { isAdmin, isLoading } = useMe();

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", mt: 8 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (!isAdmin) {
    return (
      <Alert severity="error">
        Forbidden: this page requires the admin role.
      </Alert>
    );
  }

  return <>{children}</>;
}
