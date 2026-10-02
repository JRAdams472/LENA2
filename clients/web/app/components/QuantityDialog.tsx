"use client";

import { useState } from "react";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import IconButton from "@mui/material/IconButton";
import TextField from "@mui/material/TextField";
import KeyboardArrowUpIcon from "@mui/icons-material/KeyboardArrowUp";
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown";

interface QuantityDialogProps {
  open: boolean;
  title: string;
  label?: string;
  confirmLabel?: string;
  initialValue?: number;
  min?: number;
  onClose: () => void;
  onConfirm: (quantity: number) => void;
}

export default function QuantityDialog({
  open,
  title,
  label = "Quantity",
  confirmLabel = "Add",
  initialValue = 1,
  min = 1,
  onClose,
  onConfirm,
}: QuantityDialogProps) {
  const [qty, setQty] = useState("1");
  const [source, setSource] = useState({ open, initialValue, min });

  if (
    source.open !== open ||
    source.initialValue !== initialValue ||
    source.min !== min
  ) {
    setSource({ open, initialValue, min });
    if (open) setQty(String(Math.max(min, Math.floor(initialValue))));
  }

  const value = parseInt(qty, 10);
  const valid = !isNaN(value) && value >= min;
  const bump = (delta: number) =>
    setQty(String(Math.max(min, (isNaN(value) ? 0 : value) + delta)));

  return (
    <Dialog open={open} onClose={onClose} maxWidth="xs" fullWidth>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, mt: 1 }}>
          <TextField
            label={label}
            fullWidth
            autoFocus
            value={qty}
            onChange={(e) => setQty(e.target.value.replace(/\D/g, ""))}
            onKeyDown={(e) => {
              if (e.key === "ArrowUp") bump(1);
              if (e.key === "ArrowDown") bump(-1);
            }}
            slotProps={{ htmlInput: { inputMode: "numeric" } }}
          />
          <Box sx={{ display: "flex", flexDirection: "column" }}>
            <IconButton
              size="small"
              aria-label="Increase quantity"
              onClick={() => bump(1)}
            >
              <KeyboardArrowUpIcon />
            </IconButton>
            <IconButton
              size="small"
              aria-label="Decrease quantity"
              onClick={() => bump(-1)}
              disabled={!valid || value <= min}
            >
              <KeyboardArrowDownIcon />
            </IconButton>
          </Box>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          disabled={!valid}
          onClick={() => onConfirm(value)}
        >
          {confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
