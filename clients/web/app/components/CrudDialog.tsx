"use client";

import { ReactNode, useState } from "react";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import Alert from "@mui/material/Alert";
import FormControlLabel from "@mui/material/FormControlLabel";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";
import useMediaQuery from "@mui/material/useMediaQuery";
import { useTheme } from "@mui/material/styles";

export interface FieldDef<T = Record<string, unknown>> {
  key: Extract<keyof T, string>;
  label: string;
  type?: "text" | "number" | "boolean" | "date";
  sortable?: boolean;
  render?: (row: T) => ReactNode;
  // Table-only hints — ignored by the dialog.
  minWidth?: number;
}

interface CrudDialogProps<T> {
  open: boolean;
  title: string;
  fields: FieldDef<T>[];
  values: Record<string, unknown>;
  error?: Error | null;
  // Rendered after the generated fields — for pickers that don't fit the
  // text/number/boolean/date shape (their state lives in the parent).
  extraFields?: ReactNode;
  onClose: () => void;
  onSave: (values: Record<string, unknown>) => void;
}

// A value-semantic signature for the supplied `values`: parents routinely
// rebuild the object per render (`{ ...row }`), so comparing by identity
// wiped in-progress typing on every unrelated rerender. Reset the form
// only on an open/close transition or when the values actually differ.
function valuesSignature(v: Record<string, unknown>): string {
  return JSON.stringify(v, (_key, val: unknown) =>
    val !== null && typeof val === "object" && !Array.isArray(val)
      ? Object.fromEntries(
          Object.entries(val as Record<string, unknown>).sort(([a], [b]) =>
            a.localeCompare(b)
          )
        )
      : val
  );
}

export default function CrudDialog<T = Record<string, unknown>>({
  open,
  title,
  fields,
  values,
  error,
  extraFields,
  onClose,
  onSave,
}: CrudDialogProps<T>) {
  const [form, setForm] = useState<Record<string, unknown>>(() => ({
    ...values,
  }));
  const [source, setSource] = useState(() => ({ open, sig: valuesSignature(values) }));
  const sig = valuesSignature(values);

  if (source.open !== open || source.sig !== sig) {
    setSource({ open, sig });
    setForm({ ...values });
  }

  const setValue = (key: string, value: unknown) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const theme = useTheme();
  // Phone-sized viewports get the full screen — a floating dialog leaves
  // too little room for fields, and the keyboard pushes buttons away.
  const fullScreen = useMediaQuery(theme.breakpoints.down("sm"));

  return (
    <Dialog open={open} onClose={onClose} fullWidth fullScreen={fullScreen} maxWidth="sm">
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
        {extraFields}
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
