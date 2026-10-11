import { lineClamp } from "@/lib/themeVars";

describe("lineClamp", () => {
  it("clamps at the requested line count", () => {
    expect(lineClamp(2)).toEqual({
      display: "-webkit-box",
      WebkitBoxOrient: "vertical",
      WebkitLineClamp: 2,
      overflow: "hidden",
    });
    expect(lineClamp(3).WebkitLineClamp).toBe(3);
  });
});
