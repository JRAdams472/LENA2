"use client";

import { useState } from "react";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import Alert from "@mui/material/Alert";
import FormControlLabel from "@mui/material/FormControlLabel";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";

export interface FieldDef<T = Record<string, unknown>> {
  key: Extract<keyof T, string>;
  label: string;
  type?: "text" | "number" | "boolean" | "date";
  sortable?: boolean;
}

interface CrudDialogProps<T> {
  open: boolean;
  title: string;
  fields: FieldDef<T>[];
  values: Record<string, unknown>;
  error?: Error | null;
  onClose: () => void;
  onSave: (values: Record<string, unknown>) => void;
}

export default function CrudDialog<T = Record<string, unknown>>({
  open,
  title,
  fields,
  values,
  error,
  onClose,
  onSave,
}: CrudDialogProps<T>) {
  const [form, setForm] = useState<Record<string, unknown>>(() => ({
    ...values,
  }));
  const [source, setSource] = useState({ values, open });

  if (source.values !== values || source.open !== open) {
    setSource({ values, open });
    setForm({ ...values });
  }

  const setValue = (key: string, value: unknown) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        {error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {error.message}
          </Alert>
        )}
        {fields.map((field) => {
          const type = field.type ?? "text";
          const val = form[field.key];

          if (type === "boolean") {
            return (
              <FormControlLabel
                key={field.key}
                control={
                  <Switch
                    checked={Boolean(val)}
                    onChange={(e) => setValue(field.key, e.target.checked)}
                  />
                }
                label={field.label}
                sx={{ display: "flex", mt: 1 }}
              />
            );
          }

          if (type === "number") {
            return (
              <TextField
                key={field.key}
                label={field.label}
                type="number"
                fullWidth
                margin="dense"
                value={val ?? ""}
                onChange={(e) =>
                  setValue(
                    field.key,
                    e.target.value === "" ? "" : Number(e.target.value)
                  )
                }
              />
            );
          }

          if (type === "date") {
            return (
              <TextField
                key={field.key}
                label={field.label}
                type="date"
                fullWidth
                margin="dense"
                value={val ?? ""}
                onChange={(e) => setValue(field.key, e.target.value)}
              />
            );
          }

          return (
            <TextField
              key={field.key}
              label={field.label}
              fullWidth
              margin="dense"
              value={val ?? ""}
              onChange={(e) => setValue(field.key, e.target.value)}
            />
          );
        })}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button onClick={() => onSave(form)} variant="contained">
          Save
        </Button>
      </DialogActions>
    </Dialog>
  );
}
