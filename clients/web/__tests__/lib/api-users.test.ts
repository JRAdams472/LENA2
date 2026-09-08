import { api } from "@/lib/api";

const mockFetch = global.fetch as jest.Mock;

function mockGraphQL(data: object | null, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: {
      get: (name: string) =>
        name === "content-type" ? "application/json" : null,
    },
    json: async () => ({ data }),
  };
}

function lastRequestBody() {
  const calls = mockFetch.mock.calls;
  const [, init] = calls[calls.length - 1];
  return JSON.parse((init as RequestInit).body as string);
}

const gqlUser = {
  id: "2",
  email: "member@example.com",
  displayName: "Member",
  firstName: "Ada",
  lastName: "L",
  backupEmail: "alt@example.com",
  role: "member",
  isActive: true,
  isProtected: false,
  lastLoginAt: "2026-09-01T10:00:00Z",
};

describe("api client: user management", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("getUsers pages and maps users", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        users: {
          items: [gqlUser],
          pageInfo: { pageNumber: 2, pageSize: 25, totalCount: 27 },
        },
      })
    );

    const page = await api.getUsers(2, 25);
    const body = lastRequestBody();
    expect(body.query).toContain("users(page: $page");
    expect(body.variables).toEqual({ page: 2, pageSize: 25 });
    expect(page.totalCount).toBe(27);
    expect(page.items).toHaveLength(1);
    expect(page.items[0]).toMatchObject({
      userID: 2,
      email: "member@example.com",
      firstName: "Ada",
      role: "member",
      isActive: true,
      isProtected: false,
    });
  });

  it("updateMyProfile sends the input", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ updateMyProfile: gqlUser })
    );

    const u = await api.updateMyProfile({
      firstName: "Ada",
      lastName: "L",
      backupEmail: "alt@example.com",
    });
    const body = lastRequestBody();
    expect(body.query).toContain("updateMyProfile");
    expect(body.variables.input).toEqual({
      firstName: "Ada",
      lastName: "L",
      backupEmail: "alt@example.com",
    });
    expect(u.firstName).toBe("Ada");
  });

  it("setUserRole sends userId and role", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ setUserRole: { ...gqlUser, role: "admin" } })
    );

    const u = await api.setUserRole(2, "admin");
    const body = lastRequestBody();
    expect(body.variables).toEqual({ userId: "2", role: "admin" });
    expect(u.role).toBe("admin");
  });

  it("setUserActive sends the ban flag", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ setUserActive: { ...gqlUser, isActive: false } })
    );

    const u = await api.setUserActive(2, false);
    const body = lastRequestBody();
    expect(body.variables).toEqual({ userId: "2", isActive: false });
    expect(u.isActive).toBe(false);
  });
});
