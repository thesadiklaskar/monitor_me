import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, requestJSON, sameOriginMutation } from "@/lib/server-api";

export const dynamic = "force-dynamic";

export async function GET(_request: NextRequest) {
  return backendResponse(await callBackend("/v1/dashboards", {}));
}

export async function POST(request: NextRequest) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const body = await requestJSON(request);
  if (!body || typeof body !== "object" || Array.isArray(body)) {
    return errorResponse(400, "invalid_request", "A dashboard name is required.");
  }
  return backendResponse(await callBackend("/v1/dashboards", { method: "POST", body }));
}
