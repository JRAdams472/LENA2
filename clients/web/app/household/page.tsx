"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import CircularProgress from "@mui/material/CircularProgress";
import IconButton from "@mui/material/IconButton";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import Menu from "@mui/material/Menu";
import MenuItem from "@mui/material/MenuItem";
import Paper from "@mui/material/Paper";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import MoreVertIcon from "@mui/icons-material/MoreVert";
import { api, ApiError } from "@/lib/api";
import { HouseholdMember, HouseholdRole, HouseholdUser } from "@/lib/types";
import { useMe } from "@/app/auth/useMe";

const MAX_MEMBERS = 10;

function userName(u: HouseholdUser): string {
  return (
    u.displayName ??
    [u.firstName, u.lastName].filter(Boolean).join(" ") ??
    `User ${u.userID}`
  );
}

function roleChipColor(role: HouseholdRole): "primary" | "secondary" | "default" {
  if (role === "OWNER") return "primary";
  if (role === "ADMIN") return "secondary";
  return "default";
}

export default function HouseholdPage() {
  const queryClient = useQueryClient();
  const { me } = useMe();
  const [term, setTerm] = useState("");
  const [searchTerm, setSearchTerm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [nameDraft, setNameDraft] = useState<string | null>(null);
  const [menuFor, setMenuFor] = useState<HouseholdMember | null>(null);
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);

  const householdQuery = useQuery({
    queryKey: ["myHousehold"],
    queryFn: () => api.getMyHousehold(),
  });
  const invitesQuery = useQuery({
    queryKey: ["householdInvites"],
    queryFn: () => api.getHouseholdInvites(),
  });
  const searchQuery = useQuery({
    queryKey: ["searchHouseholdUsers", searchTerm],
    queryFn: () => api.searchHouseholdUsers(searchTerm),
    enabled: searchTerm.trim().length >= 2,
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["myHousehold"] });
    queryClient.invalidateQueries({ queryKey: ["householdInvites"] });
    queryClient.invalidateQueries({ queryKey: ["searchHouseholdUsers"] });
    queryClient.invalidateQueries({ queryKey: ["unreadNotificationCount"] });
    queryClient.invalidateQueries({ queryKey: ["myNotifications"] });
    queryClient.invalidateQueries({ queryKey: ["me"] });
  };
  const onErr = (e: unknown) =>
    setError(e instanceof ApiError ? e.message : "Request failed");

  const inviteMutation = useMutation({
    mutationFn: (userID: number) => api.inviteHouseholdMember(userID),
    onSuccess: () => {
      setError(null);
      invalidate();
    },
    onError: onErr,
  });
  const acceptMutation = useMutation({
    mutationFn: (inviteID: number) => api.acceptHouseholdInvite(inviteID),
    onSuccess: () => {
      setError(null);
      invalidate();
    },
    onError: onErr,
  });
  const declineMutation = useMutation({
    mutationFn: (inviteID: number) => api.declineHouseholdInvite(inviteID),
    onSuccess: () => {
      setError(null);
      invalidate();
    },
    onError: onErr,
  });
  const cancelMutation = useMutation({
    mutationFn: (inviteID: number) => api.cancelHouseholdInvite(inviteID),
    onSuccess: () => {
      setError(null);
      invalidate();
    },
    onError: onErr,
  });
  const leaveMutation = useMutation({
    mutationFn: () => api.leaveHousehold(),
    onSuccess: () => {
      setError(null);
      invalidate();
    },
    onError: onErr,
  });
  const renameMutation = useMutation({
    mutationFn: (name: string) => api.renameHousehold(name),
    onSuccess: () => {
      setError(null);
      setNameDraft(null);
      invalidate();
    },
    onError: onErr,
  });
  const setRoleMutation = useMutation({
    mutationFn: ({ userID, role }: { userID: number; role: HouseholdRole }) =>
      api.setHouseholdRole(userID, role),
    onSuccess: () => {
      setError(null);
      closeMenu();
      invalidate();
    },
    onError: (e) => {
      closeMenu();
      onErr(e);
    },
  });
  const removeMutation = useMutation({
    mutationFn: (userID: number) => api.removeHouseholdMember(userID),
    onSuccess: () => {
      setError(null);
      closeMenu();
      invalidate();
    },
    onError: (e) => {
      closeMenu();
      onErr(e);
    },
  });
  const transferMutation = useMutation({
    mutationFn: (userID: number) => api.transferHouseholdOwnership(userID),
    onSuccess: () => {
      setError(null);
      closeMenu();
      invalidate();
    },
    onError: (e) => {
      closeMenu();
      onErr(e);
    },
  });

  function closeMenu() {
    setMenuFor(null);
    setMenuAnchor(null);
  }

  if (householdQuery.isLoading || invitesQuery.isLoading || !me) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", mt: 8 }}>
        <CircularProgress />
      </Box>
    );
  }
  if (householdQuery.error)
    return (
      <Alert severity="error">{(householdQuery.error as Error).message}</Alert>
    );
  if (invitesQuery.error)
    return (
      <Alert severity="error">{(invitesQuery.error as Error).message}</Alert>
    );

  const household = householdQuery.data;
  const invites = invitesQuery.data ?? [];
  const incoming = invites.filter((i) => i.toUser.userID === me.userID);
  const outgoing = invites.filter((i) => i.fromUser.userID === me.userID);
  const memberCount = household?.members.length ?? 0;
  const isOwner = household?.myRole === "OWNER";
  const isAdmin = household?.myRole === "ADMIN";
  const atCap = memberCount >= MAX_MEMBERS;

  // Mirrors the backend matrix: owner acts on admins/members, admin acts on
  // members only, nobody acts on the owner or themselves.
  const canManage = (m: HouseholdMember): boolean => {
    if (m.isMe || m.role === "OWNER") return false;
    if (isOwner) return true;
    return isAdmin && m.role === "MEMBER";
  };

  return (
    <Box sx={{ maxWidth: 640, display: "flex", flexDirection: "column", gap: 3 }}>
      <Typography variant="h4">Household</Typography>
      {error && <Alert severity="error">{error}</Alert>}

      <Paper sx={{ p: 2 }}>
        {isOwner ? (
          <Box
            component="form"
            onSubmit={(e) => {
              e.preventDefault();
              renameMutation.mutate(nameDraft ?? "");
            }}
            sx={{ display: "flex", gap: 1, mb: 1 }}
          >
            <TextField
              label="Household name"
              value={nameDraft ?? household?.name ?? ""}
              onChange={(e) => setNameDraft(e.target.value)}
              size="small"
              fullWidth
              slotProps={{ htmlInput: { maxLength: 100 } }}
            />
            <Button
              type="submit"
              variant="outlined"
              disabled={renameMutation.isPending}
            >
              Save
            </Button>
          </Box>
        ) : (
          household?.name && (
            <Typography variant="h6" gutterBottom>
              {household.name}
            </Typography>
          )
        )}
        <Typography variant="h6" gutterBottom>
          Members
        </Typography>
        <Typography variant="body2" color="text.secondary" gutterBottom>
          {memberCount} of {MAX_MEMBERS} members
        </Typography>
        <List dense>
          {(household?.members ?? []).map((m) => (
            <ListItem
              key={m.user.userID}
              secondaryAction={
                canManage(m) ? (
                  <IconButton
                    aria-label={`Manage ${userName(m.user)}`}
                    onClick={(e) => {
                      setMenuFor(m);
                      setMenuAnchor(e.currentTarget);
                    }}
                  >
                    <MoreVertIcon />
                  </IconButton>
                ) : undefined
              }
            >
              <ListItemText
                primary={
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                    {userName(m.user)}
                    <Chip
                      label={m.role.toLowerCase()}
                      size="small"
                      color={roleChipColor(m.role)}
                    />
                    {m.isMe && (
                      <Typography variant="body2" color="text.secondary">
                        you
                      </Typography>
                    )}
                  </Box>
                }
              />
            </ListItem>
          ))}
        </List>
        <Menu
          anchorEl={menuAnchor}
          open={menuAnchor !== null}
          onClose={closeMenu}
        >
          {menuFor && isOwner && (
            <MenuItem
              onClick={() =>
                setRoleMutation.mutate({
                  userID: menuFor.user.userID,
                  role: menuFor.role === "ADMIN" ? "MEMBER" : "ADMIN",
                })
              }
            >
              {menuFor.role === "ADMIN" ? "Demote to member" : "Make admin"}
            </MenuItem>
          )}
          {menuFor && isOwner && (
            <MenuItem
              onClick={() => {
                if (
                  window.confirm(
                    `Transfer ownership to ${userName(menuFor.user)}? You will become a regular member.`
                  )
                ) {
                  transferMutation.mutate(menuFor.user.userID);
                } else {
                  closeMenu();
                }
              }}
            >
              Transfer ownership
            </MenuItem>
          )}
          {menuFor && canManage(menuFor) && (
            <MenuItem
              onClick={() => {
                if (
                  window.confirm(
                    `Remove ${userName(menuFor.user)} from the household? They get a fresh empty pantry; their shared data stays here.`
                  )
                ) {
                  removeMutation.mutate(menuFor.user.userID);
                } else {
                  closeMenu();
                }
              }}
            >
              Remove from household
            </MenuItem>
          )}
        </Menu>
        {memberCount > 1 && (
          <Button
            color="error"
            onClick={() => {
              if (
                window.confirm(
                  "Leave this household? You will get a fresh empty pantry, cellar, meal plans, and grocery lists; your data stays with the household."
                )
              ) {
                leaveMutation.mutate();
              }
            }}
            disabled={leaveMutation.isPending}
          >
            Leave household
          </Button>
        )}
      </Paper>

      {incoming.length > 0 && (
        <Paper sx={{ p: 2 }}>
          <Typography variant="h6" gutterBottom>
            Invitations
          </Typography>
          <List dense>
            {incoming.map((inv) => (
              <ListItem
                key={inv.inviteID}
                secondaryAction={
                  <Box sx={{ display: "flex", gap: 1 }}>
                    <Button
                      size="small"
                      variant="contained"
                      onClick={() => acceptMutation.mutate(inv.inviteID)}
                      disabled={acceptMutation.isPending}
                    >
                      Accept
                    </Button>
                    <Button
                      size="small"
                      onClick={() => declineMutation.mutate(inv.inviteID)}
                      disabled={declineMutation.isPending}
                    >
                      Decline
                    </Button>
                  </Box>
                }
              >
                <ListItemText
                  primary={`${userName(inv.fromUser)} invited you`}
                  secondary="Joining merges your pantry, cellar, meal plans, and grocery lists into theirs."
                />
              </ListItem>
            ))}
          </List>
        </Paper>
      )}

      {outgoing.length > 0 && (
        <Paper sx={{ p: 2 }}>
          <Typography variant="h6" gutterBottom>
            Sent invitations
          </Typography>
          <List dense>
            {outgoing.map((inv) => (
              <ListItem
                key={inv.inviteID}
                secondaryAction={
                  <Button
                    size="small"
                    onClick={() => cancelMutation.mutate(inv.inviteID)}
                    disabled={cancelMutation.isPending}
                  >
                    Cancel
                  </Button>
                }
              >
                <ListItemText primary={userName(inv.toUser)} secondary="Pending" />
              </ListItem>
            ))}
          </List>
        </Paper>
      )}

      <Paper sx={{ p: 2 }}>
        <Typography variant="h6" gutterBottom>
          Invite someone
        </Typography>
        {atCap && (
          <Alert severity="info" sx={{ mb: 2 }}>
            This household is at the {MAX_MEMBERS}-member limit.
          </Alert>
        )}
        <Box
          component="form"
          onSubmit={(e) => {
            e.preventDefault();
            setSearchTerm(term);
          }}
          sx={{ display: "flex", gap: 1, mb: 2 }}
        >
          <TextField
            label="Name or email"
            value={term}
            onChange={(e) => setTerm(e.target.value)}
            size="small"
            fullWidth
          />
          <Button type="submit" variant="outlined">
            Search
          </Button>
        </Box>
        {searchTerm.trim().length > 0 && searchTerm.trim().length < 2 && (
          <Typography color="text.secondary">
            Type at least 2 characters to search.
          </Typography>
        )}
        {searchQuery.isLoading && <CircularProgress size={24} />}
        <List dense>
          {(searchQuery.data ?? []).map((u) => (
            <ListItem
              key={u.userID}
              secondaryAction={
                <Button
                  size="small"
                  variant="contained"
                  onClick={() => inviteMutation.mutate(u.userID)}
                  disabled={inviteMutation.isPending || atCap}
                >
                  Invite
                </Button>
              }
            >
              <ListItemText primary={userName(u)} />
            </ListItem>
          ))}
        </List>
        {searchQuery.data && searchQuery.data.length === 0 && (
          <Typography color="text.secondary">No users found.</Typography>
        )}
      </Paper>
    </Box>
  );
}
