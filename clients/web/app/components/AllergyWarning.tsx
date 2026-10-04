"use client";

import Alert from "@mui/material/Alert";
import Chip from "@mui/material/Chip";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import { SxProps, Theme } from "@mui/material/styles";
import WarningAmberIcon from "@mui/icons-material/WarningAmber";
import ReportIcon from "@mui/icons-material/Report";
import { AllergenFlag, AllergyWarning } from "@/lib/types";

const memberName = (w: AllergyWarning): string =>
  w.member.displayName ||
  [w.member.firstName, w.member.lastName].filter(Boolean).join(" ") ||
  "Household member";

const describe = (w: AllergyWarning): string =>
  `${memberName(w)} — ${w.allergen.name} (${w.memberKind}; ${w.entityKind === "contains" ? "contains" : "may contain"})`;

// Confirmed allergen against a true allergy record is the severe case;
// advisory flags and dietary preferences stay at warning level.
const isSevere = (warnings: AllergyWarning[]): boolean =>
  warnings.some((w) => w.memberKind === "allergy" && w.entityKind === "contains");

export function AllergyWarningChip({
  warnings,
  size = "small",
  sx,
}: {
  warnings?: AllergyWarning[];
  size?: "small" | "medium";
  sx?: SxProps<Theme>;
}) {
  if (!warnings || warnings.length === 0) return null;
  const severe = isSevere(warnings);
  const label = `${warnings.length} allergy ${warnings.length === 1 ? "warning" : "warnings"}`;
  return (
    <Tooltip
      title={
        <>
          {warnings.map((w) => (
            <Typography
              key={`${w.member.userID}-${w.allergen.allergenID}`}
              variant="caption"
              sx={{ display: "block" }}
            >
              {describe(w)}
            </Typography>
          ))}
        </>
      }
    >
      <Chip
        icon={severe ? <ReportIcon /> : <WarningAmberIcon />}
        label={label}
        color={severe ? "error" : "warning"}
        variant="outlined"
        size={size}
        sx={sx}
        data-testid="allergy-warning-chip"
      />
    </Tooltip>
  );
}

// Detail-page form: lists each member conflict in an Alert rather than
// hiding names behind a tooltip.
export function AllergyWarningsAlert({
  warnings,
  sx,
}: {
  warnings?: AllergyWarning[];
  sx?: SxProps<Theme>;
}) {
  if (!warnings || warnings.length === 0) return null;
  return (
    <Alert
      severity={isSevere(warnings) ? "error" : "warning"}
      sx={sx}
      data-testid="allergy-warning-alert"
    >
      {warnings.map((w) => (
        <Typography
          key={`${w.member.userID}-${w.allergen.allergenID}`}
          variant="body2"
          sx={{ display: "block" }}
        >
          {describe(w)}
        </Typography>
      ))}
    </Alert>
  );
}

// Allergen flags for detail views. An empty set is "no information" —
// curated data is incomplete, so absence must never read as "safe".
export function AllergenFlagsLine({ allergens }: { allergens?: AllergenFlag[] }) {
  if (!allergens || allergens.length === 0) {
    return (
      <Typography variant="caption" color="text.secondary" data-testid="allergen-flags-empty">
        No allergen information
      </Typography>
    );
  }
  return (
    <>
      {allergens.map((f) => (
        <Chip
          key={f.allergen.allergenID}
          label={f.kind === "contains" ? f.allergen.name : `${f.allergen.name} (may contain)`}
          size="small"
          variant="outlined"
          color={f.kind === "contains" ? "default" : "warning"}
          sx={{ mr: 0.5, mb: 0.5 }}
        />
      ))}
    </>
  );
}
