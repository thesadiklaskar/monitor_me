import { NextRequest } from "next/server";
import { backendResponse, callBackend } from "@/lib/server-api";
export const dynamic = "force-dynamic";
export async function GET() {
  return backendResponse(await callBackend("/v1/notifications", {}));
}
