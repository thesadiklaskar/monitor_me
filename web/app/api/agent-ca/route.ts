import { readFile } from "node:fs/promises";
import { NextResponse } from "next/server";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET() {
  const certificatePath = process.env.MONITOR_ME_AGENT_CA_FILE;
  if (!certificatePath) {
    return NextResponse.json(
      { error: "agent_ca_unavailable", message: "The control-plane agent CA path is not configured." },
      { status: 503, headers: { "Cache-Control": "no-store" } },
    );
  }

  try {
    const certificate = await readFile(certificatePath);
    const text = certificate.toString("utf8");
    if (!text.includes("-----BEGIN CERTIFICATE-----")) {
      throw new Error("Configured agent CA file is not PEM encoded");
    }
    return new Response(new Uint8Array(certificate), {
      headers: {
        "Content-Type": "application/x-pem-file",
        "Content-Length": String(certificate.byteLength),
        "Content-Disposition": 'attachment; filename="monitor-me-agent-ca.pem"',
        "Cache-Control": "no-store",
        "X-Content-Type-Options": "nosniff",
      },
    });
  } catch {
    return NextResponse.json(
      { error: "agent_ca_unavailable", message: "The control-plane agent CA certificate cannot be read." },
      { status: 503, headers: { "Cache-Control": "no-store" } },
    );
  }
}
