import Link from "next/link";
import Typography from "@mui/material/Typography";

// LEN-31: the standing "check the label" reminder shown on surfaces that
// display allergen/dietary information. Links to the full public text.
export default function DisclaimerLine() {
  return (
    <Typography variant="caption" color="text.secondary" component="p">
      Allergy and dietary information is a convenience, not medical advice —
      always check product labels.{" "}
      <Link href="/disclaimer">Medical &amp; allergen disclaimer</Link>
    </Typography>
  );
}
