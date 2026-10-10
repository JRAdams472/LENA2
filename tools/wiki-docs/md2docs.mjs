// Author-time wiki → docs page transpiler. Reads a wiki .md file and emits a
// page.tsx that uses the shared DocScaffold primitives. Run from clients/web:
//   node .md2docs.mjs <wikiMdPath> <slug> <title>
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { dirname } from "node:path";

const [, , mdPath, slug, titleArg] = process.argv;
if (!mdPath || !slug) {
  console.error("usage: node .md2docs.mjs <wiki.md> <slug> [title]");
  process.exit(1);
}

const SLUGS = {
  Home: "/docs",
  "Getting-Started": "/docs/getting-started",
  "Tips-and-FAQ": "/docs/tips-faq",
  "Dashboard-and-Notifications": "/docs/dashboard",
  Recipes: "/docs/recipes",
  "Meal-Planning": "/docs/meal-planning",
  "Grocery-Lists": "/docs/grocery-lists",
  "Food-Events": "/docs/food-events",
  "Inventory-and-Pantry": "/docs/pantry",
  Ingredients: "/docs/ingredients",
  "Allergy-Tracking": "/docs/allergy-tracking",
  "Wine-Cellar": "/docs/wine-cellar",
  Households: "/docs/households",
  "AI-Assistant": "/docs/ai-assistant",
  "Analytics-and-Recommendations": "/docs/analytics",
  "Privacy-and-Local-AI": "/docs/privacy",
  Themes: "/docs/themes",
  "Mobile-App": "/docs/mobile-app",
  "Mobile-Dashboard": "/docs/mobile-dashboard",
  "Mobile-Recipes-and-Meal-Plans": "/docs/mobile-recipes",
  "Mobile-Cook-Mode": "/docs/mobile-cook-mode",
  "Mobile-Grocery-Lists": "/docs/mobile-grocery",
  "Mobile-Events": "/docs/mobile-events",
  "Mobile-Pantry": "/docs/mobile-pantry",
  "Mobile-Scanning": "/docs/mobile-scanning",
  "Mobile-Allergy-Tracking": "/docs/mobile-allergies",
  "Mobile-Wine": "/docs/mobile-wine",
  "Mobile-Household": "/docs/mobile-household",
  "Mobile-Ask-Dot": "/docs/mobile-ask-dot",
  "Mobile-Tablet-Layouts": "/docs/mobile-tablet",
};

const md = readFileSync(mdPath, "utf8").replace(/\r\n/g, "\n");
const lines = md.split("\n");

// ---- inline markdown → JSX string ----
// Plain-text strip for intros/table heads: removes md formatting entirely.
function plain(s) {
  return s
    .replace(/\[([^\]]+)\]\([^)]+\)/g, "$1")
    .replace(/\*\*([^*]+)\*\*/g, "$1")
    .replace(/\*([^*]+)\*/g, "$1")
    .replace(/`([^`]+)`/g, "$1");
}
function inline(src, inArray = false) {
  // tokenize: links first, then bold/italic/code
  const out = [];
  let rest = src;
  const re =
    /\[([^\]]+)\]\(([^)\s]+)\)|\*\*([^*]+)\*\*|\*([^*]+)\*|`([^`]+)`|([^[\]*`]+)/g;
  let m;
  let key = 0;
  while ((m = re.exec(rest))) {
    if (m[1] !== undefined) {
      const text = m[1];
      let href = m[2];
      if (!href.startsWith("http") && !href.startsWith("/")) {
        const [page, anchor] = href.replace(/\.md$/, "").split("#");
        href = SLUGS[page] ?? `/docs/${page.toLowerCase()}`;
        if (anchor) href += `#${anchor}`;
      }
      const external = href.startsWith("http");
      out.push(
        `<DocLink key={${key++}} href="${href}"${external ? " external" : ""}>${jsxText(text)}</DocLink>`
      );
    } else if (m[3] !== undefined) {
      out.push(`<strong key={${key++}}>${jsxText(m[3])}</strong>`);
    } else if (m[4] !== undefined) {
      out.push(`<em key={${key++}}>${jsxText(m[4])}</em>`);
    } else if (m[5] !== undefined) {
      out.push(`<code key={${key++}}>${jsxText(m[5])}</code>`);
    } else if (m[6] !== undefined) {
      out.push(jsxText(m[6]));
    }
  }
  const frag = `<Fragment key={${fragKey++}}>${out.join("")}</Fragment>`;
  return inArray ? frag : `{${frag}}`;
}
function jsxText(s) {
  // Emits a JS string child {"..."} — no HTML entities, only JS escaping.
  return `{"${s.replace(/\\/g, "\\\\").replace(/"/g, '\\"').replace(/\n/g, " ")}"}`;
}

// ---- block parsing ----
const blocks = []; // {type, ...}
let i = 0;
while (i < lines.length) {
  const line = lines[i];
  if (/^#{1}\s/.test(line)) {
    blocks.push({ type: "h1", text: line.slice(2).trim() });
    i++;
  } else if (/^#{2}\s/.test(line)) {
    blocks.push({ type: "h2", text: line.slice(3).trim() });
    i++;
  } else if (/^#{3,}\s/.test(line)) {
    blocks.push({ type: "h3", text: line.replace(/^#+\s*/, "").trim() });
    i++;
  } else if (/^!\[/.test(line)) {
    const m = line.match(/^!\[([^\]]*)\]\(([^)]+)\)/);
    if (m) blocks.push({ type: "img", alt: m[1], src: m[2] });
    i++;
  } else if (/^>\s?/.test(line)) {
    const buf = [];
    while (i < lines.length && /^>\s?/.test(lines[i])) {
      buf.push(lines[i].replace(/^>\s?/, ""));
      i++;
    }
    blocks.push({ type: "quote", text: buf.join(" ") });
  } else if (/^\|/.test(line)) {
    const rows = [];
    while (i < lines.length && /^\|/.test(lines[i])) {
      const cells = lines[i].replace(/^\||\|$/g, "").split("|").map((c) => c.trim());
      if (!/^[:\-\s|]+$/.test(lines[i])) rows.push(cells);
      i++;
    }
    blocks.push({ type: "table", head: rows[0] ?? [], rows: rows.slice(1) });
  } else if (/^\s*[-*]\s+/.test(line)) {
    const items = [];
    while (i < lines.length && /^\s*[-*]\s+/.test(lines[i])) {
      items.push(lines[i].replace(/^\s*[-*]\s+/, ""));
      i++;
    }
    blocks.push({ type: "ul", items });
  } else if (/^\s*\d+\.\s+/.test(line)) {
    const items = [];
    while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) {
      items.push(lines[i].replace(/^\s*\d+\.\s+/, ""));
      i++;
    }
    blocks.push({ type: "ol", items });
  } else if (line.trim() === "" || /^---+$/.test(line)) {
    i++;
  } else {
    // paragraph: consume until blank/other block
    const buf = [line.trim()];
    i++;
    while (
      i < lines.length &&
      lines[i].trim() !== "" &&
      !/^(#{1,}\s|!|\||>|\s*[-*]\s|\s*\d+\.\s)/.test(lines[i])
    ) {
      buf.push(lines[i].trim());
      i++;
    }
    blocks.push({ type: "p", text: buf.join(" ") });
  }
}

// ---- emit ----
const title = titleArg ?? (blocks[0]?.type === "h1" ? blocks[0].text : slug);
// first paragraph becomes the intro
let intro = "";
const bodyBlocks = blocks.filter((b) => {
  if (b.type === "h1") return false;
  if (!intro && b.type === "p") {
    intro = plain(b.text);
    return false;
  }
  return true;
});
if (!intro) intro = "";

const device = (src) =>
  /tablet/.test(src) ? "tablet" : /mobile|phone/.test(src) ? "phone" : "web";

let fragKey = 0;

const parts = [];
let sectionOpen = false;
for (const b of bodyBlocks) {
  if (b.type === "h2") {
    if (sectionOpen) parts.push("      </Section>");
    parts.push(`      <Section title={${JSON.stringify(b.text)}}>`);
    sectionOpen = true;
  } else if (b.type === "h3") {
    parts.push(`        <DocSub>${inline(b.text)}</DocSub>`);
  } else if (b.type === "p") {
    parts.push(`        <P>${inline(b.text)}</P>`);
  } else if (b.type === "ul") {
    parts.push(
      `        <DocList items={[${b.items.map((it) => `\n          ${inline(it, true)},`).join("")}\n        ]} />`
    );
  } else if (b.type === "ol") {
    parts.push(
      `        <DocList ordered items={[${b.items.map((it) => `\n          ${inline(it, true)},`).join("")}\n        ]} />`
    );
  } else if (b.type === "img") {
    const src = b.src.replace(/^images\//, "/images/");
    parts.push(
      `        <Screenshot src={${JSON.stringify(src)}} alt={${JSON.stringify(b.alt)}} device="${device(src)}" />`
    );
  } else if (b.type === "quote") {
    parts.push(`        <Callout>${inline(b.text)}</Callout>`);
  } else if (b.type === "table") {
    const rowsJsx = b.rows.map(
      (r) => `[${r.map((c) => inline(c, true)).join(", ")}]`
    );
    parts.push(
      `        <DocTable head={${JSON.stringify(b.head.map(plain))}} rows={[${rowsJsx.join(", ")}]} />`
    );
  }
}
if (sectionOpen) parts.push("      </Section>");

const used = ["Callout", "DocLink", "DocList", "DocSub", "DocTable", "P", "Section", "Screenshot"].filter(
  (n) => parts.join("\n").includes(`<${n}`)
);
const out = `import { Fragment } from "react";
import type { Metadata } from "next";
import {
  DocPage,
  ${used.join(",\n  ")},
} from "@/app/components/marketing/DocScaffold";

export const metadata: Metadata = {
  title: ${JSON.stringify(title + " — LENA docs")},
};

export default function Page() {
  return (
    <DocPage title={${JSON.stringify(title)}} intro={${JSON.stringify(intro)}}>
${parts.join("\n")}
    </DocPage>
  );
}
`;

const outPath = `${new URL("../clients/web/app/docs/", import.meta.url).pathname.replace(/^/([A-Za-z]:)/, "$1")}${slug}/page.tsx`;
mkdirSync(dirname(outPath), { recursive: true });
writeFileSync(outPath, out);
console.log("wrote", outPath, `(${bodyBlocks.length} blocks)`);
