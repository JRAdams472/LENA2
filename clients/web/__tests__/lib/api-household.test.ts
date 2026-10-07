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

const gqlHouseholdUser = {
  id: "9",
  displayName: "Mate",
  firstName: "Ann",
  lastName: "Lee",
};

const gqlHousehold = {
  id: "42",
  name: "The Smiths",
  myRole: "OWNER",
  isActive: true,
  members: [{ user: gqlHouseholdUser, role: "MEMBER", isMe: false }],
  createdAt: "2026-01-01T00:00:00Z",
};

const gqlInvite = {
  id: "55",
  status: "PENDING",
  createdAt: "2026-09-20T10:00:00Z",
  fromUser: gqlHouseholdUser,
  toUser: { id: "7", displayName: "Me", firstName: null, lastName: null },
};

describe("api client: household", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("getMyHousehold maps members", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ myHousehold: gqlHousehold }));
    const hh = await api.getMyHousehold();
    expect(hh).toEqual({
      householdID: 42,
      name: "The Smiths",
      myRole: "OWNER",
      isActive: true,
      members: [
        {
          user: { userID: 9, displayName: "Mate", firstName: "Ann", lastName: "Lee" },
          role: "MEMBER",
          isMe: false,
        },
      ],
      createdAt: "2026-01-01T00:00:00Z",
    });
  });

  it("getMyHousehold returns null when none", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ myHousehold: null }));
    expect(await api.getMyHousehold()).toBeNull();
  });

  it("getHouseholdInvites maps invites", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ householdInvites: [gqlInvite] })
    );
    const invites = await api.getHouseholdInvites();
    expect(invites).toHaveLength(1);
    expect(invites[0].inviteID).toBe(55);
    expect(invites[0].status).toBe("PENDING");
    expect(invites[0].fromUser.userID).toBe(9);
    expect(invites[0].toUser.userID).toBe(7);
  });

  it("searchHouseholdUsers sends term and limit", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ searchHouseholdUsers: [gqlHouseholdUser] })
    );
    const users = await api.searchHouseholdUsers("ann", 5);
    const body = lastRequestBody();
    expect(body.query).toContain("searchHouseholdUsers");
    expect(body.variables).toEqual({ term: "ann", limit: 5 });
    expect(users).toHaveLength(1);
    expect(users[0].userID).toBe(9);
  });

  it("inviteHouseholdMember sends the user id as a string", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ inviteHouseholdMember: gqlInvite })
    );
    const inv = await api.inviteHouseholdMember(9);
    const body = lastRequestBody();
    expect(body.query).toContain("inviteHouseholdMember");
    expect(body.variables).toEqual({ userId: "9" });
    expect(inv.inviteID).toBe(55);
  });

  it("acceptHouseholdInvite returns the household", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ acceptHouseholdInvite: gqlHousehold })
    );
    const hh = await api.acceptHouseholdInvite(55);
    const body = lastRequestBody();
    expect(body.variables).toEqual({
      inviteId: "55",
      mergeFromHouseholdId: null,
    });
    expect(hh.householdID).toBe(42);
  });

  it("acceptHouseholdInvite sends the merge source when given", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ acceptHouseholdInvite: gqlHousehold })
    );
    await api.acceptHouseholdInvite(55, 77);
    const body = lastRequestBody();
    expect(body.variables).toEqual({
      inviteId: "55",
      mergeFromHouseholdId: "77",
    });
  });

  it("getMyHouseholds maps the list", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ myHouseholds: [gqlHousehold] })
    );
    const list = await api.getMyHouseholds();
    expect(list).toHaveLength(1);
    expect(list[0].householdID).toBe(42);
    expect(list[0].isActive).toBe(true);
  });

  it("setActiveHousehold sends the household id", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ setActiveHousehold: gqlHousehold })
    );
    const hh = await api.setActiveHousehold(77);
    const body = lastRequestBody();
    expect(body.query).toContain("setActiveHousehold");
    expect(body.variables).toEqual({ householdId: "77" });
    expect(hh.householdID).toBe(42);
  });

  it("createHousehold sends the name and maps the result", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ createHousehold: gqlHousehold })
    );
    const hh = await api.createHousehold("Cabin");
    const body = lastRequestBody();
    expect(body.variables).toEqual({ name: "Cabin" });
    expect(hh.householdID).toBe(42);
  });

  it("createHousehold sends null for a blank name", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({ createHousehold: gqlHousehold })
    );
    await api.createHousehold("  ");
    const body = lastRequestBody();
    expect(body.variables).toEqual({ name: null });
  });

  it("declineHouseholdInvite returns the invite", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        declineHouseholdInvite: { ...gqlInvite, status: "DECLINED" },
      })
    );
    const inv = await api.declineHouseholdInvite(55);
    expect(inv.status).toBe("DECLINED");
  });

  it("cancelHouseholdInvite returns the invite", async () => {
    mockFetch.mockResolvedValueOnce(
      mockGraphQL({
        cancelHouseholdInvite: { ...gqlInvite, status: "CANCELLED" },
      })
    );
    const inv = await api.cancelHouseholdInvite(55);
    expect(inv.status).toBe("CANCELLED");
  });

  it("leaveHousehold returns the boolean", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ leaveHousehold: true }));
    expect(await api.leaveHousehold()).toBe(true);
  });

  it("leaveHousehold sends the target household id", async () => {
    mockFetch.mockResolvedValueOnce(mockGraphQL({ leaveHousehold: true }));
    expect(await api.leaveHousehold(77)).toBe(true);
    const body = lastRequestBody();
    expect(body.variables).toEqual({ householdId: "77" });
  });
});
