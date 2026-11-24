import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse } from "@/lib/server-api";
export const dynamic = "force-dynamic";
export async function GET(_request: NextRequest) {
  return backendResponse(await callBackend("/v1/notifications", {}));
}
