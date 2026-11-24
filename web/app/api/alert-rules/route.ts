import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, requestJSON, sameOriginMutation } from "@/lib/server-api";
export const dynamic = "force-dynamic";
export async function GET(_request: NextRequest) {
  return backendResponse(await callBackend("/v1/alert-rules", {}));
}
export async function POST(request: NextRequest) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const body = await requestJSON(request);
  if (!body || typeof body !== "object") return errorResponse(400, "invalid_request", "Alert rule data is required.");
  return backendResponse(await callBackend("/v1/alert-rules", { method: "POST", body }));
}
