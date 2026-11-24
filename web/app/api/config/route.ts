import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

function publicAPIOrigin(): string | null {
  const configured = process.env.MONITOR_ME_PUBLIC_API_URL;
  if (!configured) return null;
  try {
    const url = new URL(configured);
    if (!["http:", "https:"].includes(url.protocol) || url.username || url.password) return null;
    return url.origin;
  } catch {
    return null;
  }
}

export async function GET() {
  return NextResponse.json(
    {
      agent_grpc_address: process.env.MONITOR_ME_AGENT_GRPC_ADDRESS || null,
      public_api_url: publicAPIOrigin(),
    },
    { headers: { "Cache-Control": "no-store" } },
  );
}
