import { ReactNode } from "react";
import AdminLayout from "@/app/components/AdminLayout";

export default function MemberLayout({ children }: { children: ReactNode }) {
  return <AdminLayout>{children}</AdminLayout>;
}
