import "@testing-library/jest-dom";
import { render, screen } from "@testing-library/react";

import AdminGate from "@/app/components/AdminGate";

jest.mock("../../app/auth/useMe");
import { useMe } from "@/app/auth/useMe";

const mockedUseMe = useMe as jest.Mock;

describe("AdminGate", () => {
  it("shows a spinner while the role is loading", () => {
    mockedUseMe.mockReturnValue({ me: null, isAdmin: false, isLoading: true });
    render(
      <AdminGate>
        <div>Secret</div>
      </AdminGate>
    );
    expect(screen.getByRole("progressbar")).toBeInTheDocument();
    expect(screen.queryByText("Secret")).not.toBeInTheDocument();
  });

  it("renders children for admins", () => {
    mockedUseMe.mockReturnValue({
      me: { role: "admin" },
      isAdmin: true,
      isLoading: false,
    });
    render(
      <AdminGate>
        <div>Secret</div>
      </AdminGate>
    );
    expect(screen.getByText("Secret")).toBeInTheDocument();
  });

  it("shows Forbidden for members and does not mount children", () => {
    mockedUseMe.mockReturnValue({
      me: { role: "member" },
      isAdmin: false,
      isLoading: false,
    });
    render(
      <AdminGate>
        <div>Secret</div>
      </AdminGate>
    );
    expect(
      screen.getByText("Forbidden: this page requires the admin role.")
    ).toBeInTheDocument();
    expect(screen.queryByText("Secret")).not.toBeInTheDocument();
  });
});
