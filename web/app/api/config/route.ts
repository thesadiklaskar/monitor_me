import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

export async function GET() {
  return NextResponse.json(
    { agent_grpc_address: process.env.MONITOR_ME_AGENT_GRPC_ADDRESS || null },
    { headers: { "Cache-Control": "no-store" } },
  );
}
