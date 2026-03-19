import assert from "node:assert/strict";
import fs from "node:fs/promises";
import test from "node:test";

const workflowPath = new URL("../../.github/workflows/ci.yml", import.meta.url);

async function readWorkflow() {
  return await fs.readFile(workflowPath, "utf8");
}

test("screenshot comment is generated before pushing the screenshot ref", async () => {
  const workflow = await readWorkflow();
  const generateIndex = workflow.indexOf("- name: Generate screenshot comment");
  const pushIndex = workflow.indexOf("- name: Push screenshots to ref");

  assert.notEqual(generateIndex, -1, "missing Generate screenshot comment step");
  assert.notEqual(pushIndex, -1, "missing Push screenshots to ref step");
  assert.ok(generateIndex < pushIndex, "comment generation must run before orphan checkout rewrites the workspace");
});

test("screenshot comment is written outside the repo workspace", async () => {
  const workflow = await readWorkflow();

  assert.match(
    workflow,
    /COMMENT_OUTPUT_PATH:\s+\$\{\{\s*runner\.temp\s*\}\}\/e2e-screenshot-comment\.md/,
  );
  assert.match(
    workflow,
    /COMMENT_PATH:\s+\$\{\{\s*runner\.temp\s*\}\}\/e2e-screenshot-comment\.md/,
  );
  assert.match(workflow, /readFileSync\(process\.env\.COMMENT_PATH,\s*"utf8"\)/);
});
