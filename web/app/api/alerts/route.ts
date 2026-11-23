import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, sessionToken } from "@/lib/server-api";
export const dynamic = "force-dynamic";
export async function GET(request: NextRequest) {
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to view alerts.");
  return backendResponse(await callBackend("/v1/alerts", { token }));
}
