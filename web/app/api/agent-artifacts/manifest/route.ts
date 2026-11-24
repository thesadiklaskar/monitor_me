import { readFile } from "node:fs/promises";
import path from "node:path";
import { NextResponse } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET() {
  const directory = process.env.MONITOR_ME_AGENT_ARTIFACT_DIR || "/app/.data/agent-artifacts";
  try {
    const content = await readFile(path.join(directory, "manifest.json"), "utf8");
    const manifest = JSON.parse(content) as {
      schema_version: number;
      version: string;
      artifacts: Array<{ os: string; arch: string; file: string; sha256: string }>;
    };
    if (manifest.schema_version !== 1 || typeof manifest.version !== "string" || !Array.isArray(manifest.artifacts)) {
      throw new Error("Invalid agent artifact manifest");
    }
    return NextResponse.json(manifest, {
      headers: { "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" },
    });
  } catch {
    return NextResponse.json(
      { error: "agent_artifacts_unavailable", message: "Native agent artifacts are not ready on this control plane." },
      { status: 503, headers: { "Cache-Control": "no-store" } },
    );
  }
}
