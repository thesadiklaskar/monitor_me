import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { NextResponse } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const supportedArtifacts: Record<string, Record<string, string>> = {
  linux: { amd64: "monitor-agent", arm64: "monitor-agent" },
  windows: { amd64: "monitor-agent.exe" },
};

type RouteContext = { params: Promise<{ os: string; arch: string }> };

export async function GET(_request: Request, context: RouteContext) {
  const { os, arch } = await context.params;
  const filename = supportedArtifacts[os]?.[arch];
  if (!filename) {
    return NextResponse.json({ error: "unsupported_agent_platform" }, { status: 404 });
  }

  const directory = process.env.MONITOR_ME_AGENT_ARTIFACT_DIR || "/app/.data/agent-artifacts";
  const relativeFile = \`\${os}-\${arch}/\${filename}\`;
  try {
    const manifestRaw = await readFile(path.join(directory, "manifest.json"), "utf8");
    const manifest = JSON.parse(manifestRaw) as {
      version: string;
      artifacts: Array<{ os: string; arch: string; file: string; sha256: string }>;
    };
    const entry = manifest.artifacts.find(
      (artifact) => artifact.os === os && artifact.arch === arch && artifact.file === relativeFile,
    );
    if (!entry || !/^[a-f0-9]{64}$/i.test(entry.sha256)) {
      throw new Error("Agent artifact is not present in the release manifest");
    }

    const content = await readFile(path.join(directory, relativeFile));
    const digest = createHash("sha256").update(content).digest("hex");
    if (digest.toLowerCase() !== entry.sha256.toLowerCase()) {
      throw new Error("Agent artifact checksum does not match the release manifest");
    }

    return new Response(new Uint8Array(content), {
      headers: {
        "Content-Type": "application/octet-stream",
        "Content-Length": String(content.byteLength),
        "Content-Disposition": \`attachment; filename="\${filename}"\`,
        "Cache-Control": "public, max-age=300",
        "X-Content-Type-Options": "nosniff",
        "X-MonitorMe-SHA256": digest,
        "X-MonitorMe-Agent-Version": manifest.version,
      },
    });
  } catch {
    return NextResponse.json(
      { error: "agent_artifact_unavailable", message: "The requested native agent artifact is not ready." },
      { status: 503, headers: { "Cache-Control": "no-store" } },
    );
  }
}
