import React from "react";
import { render as rtlRender } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider } from "@/app/auth/AuthProvider";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
});

function AllTheProviders({ children }: { children: React.ReactNode }) {
  return (
    <AuthProvider>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </AuthProvider>
  );
}

export function render(
  ui: React.ReactElement,
  options?: Omit<Parameters<typeof rtlRender>[1], "wrapper">
) {
  return {
    user: userEvent.setup(),
    ...rtlRender(ui, { wrapper: AllTheProviders, ...options }),
  };
}

// Emotion serializes `sx` into generated CSSOM rules — jsdom shows no
// inline style, so assertions walk document.styleSheets for rules the
// element matches. Returns the concatenated cssText of matching rules.
export function generatedCssFor(el: Element): string {
  let css = "";
  for (const sheet of Array.from(document.styleSheets)) {
    for (const rule of Array.from(sheet.cssRules)) {
      try {
        if (rule instanceof CSSStyleRule && el.matches(rule.selectorText)) {
          css += `${rule.cssText}\n`;
        }
      } catch {
        // selector unsupported by jsdom — skip
      }
    }
  }
  return css;
}

// True when the element or an ancestor carries a -webkit-line-clamp rule
// (the lineClamp() helper). Parent walk covers text nested inside the
// clamped wrapper, e.g. a Link inside the clamped Typography.
export function hasLineClamp(el: Element | null): boolean {
  for (let node: Element | null = el; node; node = node.parentElement) {
    if (generatedCssFor(node).includes("-webkit-line-clamp")) return true;
  }
  return false;
}
