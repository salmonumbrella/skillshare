#!/usr/bin/env node
// Markdown subagent (Claude style) → OpenCode agent (https://opencode.ai/docs/agents/).
//
// Only conversions that need no guessing:
// - keys OpenCode does not document are dropped (name, Claude-only fields, targets)
// - `mode` defaults to subagent; OpenCode's own default (all) would list a
//   helper agent as a primary agent
// - a `model` that is not provider/model-id is dropped, so OpenCode uses its default
// - `tools`, `disallowedTools` and `permissionMode` fail the conversion: they
//   restrict a Claude agent, and dropping them would let the OpenCode agent use
//   every tool

// Frontmatter keys passed through unchanged. Add OpenCode keys here as needed.
const KEEP = ["description", "mode", "model", "temperature", "top_p", "steps", "permission", "hidden", "color", "prompt"];

let input = "";
process.stdin.setEncoding("utf8");
process.stdin.on("data", (chunk) => (input += chunk));
process.stdin.on("end", () => {
  try {
    process.stdout.write(convert(input));
  } catch (err) {
    process.stderr.write(`opencode-agents: ${err.message}\n`);
    process.exit(1);
  }
});

function convert(text) {
  const match = text.match(/^---\r?\n([\s\S]*?)\r?\n---[ \t]*\r?\n?/);
  const body = match ? text.slice(match[0].length) : text;
  const entries = parseEntries(match ? match[1] : "");
  const has = (key) => entries.some((e) => e.key === key);

  const restriction = ["tools", "disallowedTools", "permissionMode"].find(has);
  if (restriction) {
    throw new Error(
      `'${restriction}' limits which tools a Claude agent may use and has no safe OpenCode equivalent. ` +
        "Keep a separate OpenCode variant with `targets: [opencode]` and `permission:`, " +
        "and add `targets:` to this agent so it skips opencode."
    );
  }
  if (!has("description")) {
    throw new Error("missing required frontmatter 'description'");
  }

  const lines = entries
    .filter((e) => KEEP.includes(e.key) && !(e.key === "model" && !e.value.includes("/")))
    .flatMap((e) => e.lines);
  if (!has("mode")) lines.push("mode: subagent");
  return `---\n${lines.join("\n")}\n---\n${body}`;
}

// parseEntries groups frontmatter lines by top-level key; indented lines
// (nested maps such as permission, or lists) stay with the key above them.
function parseEntries(text) {
  const entries = [];
  for (const line of text.split(/\r?\n/)) {
    const m = line.match(/^([A-Za-z0-9_-]+)\s*:\s*(.*)$/);
    if (m) entries.push({ key: m[1], value: m[2].trim(), lines: [line] });
    else if (entries.length) entries[entries.length - 1].lines.push(line);
  }
  return entries;
}
