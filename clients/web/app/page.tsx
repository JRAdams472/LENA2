import { redirect } from "next/navigation";

// Placeholder until the LEN-123 marketing landing lands: send everyone to
// the member app, which gates unauthenticated visitors on the login screen.
export default function RootPage() {
  redirect("/member");
}
