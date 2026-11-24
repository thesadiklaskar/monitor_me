import { NextRequest } from "next/server";
import {
  backendResponse,
  callBackend,
  errorResponse,
  requestJSON,
  sameOriginMutation,
} from "@/lib/server-api";

export const dynamic = "force-dynamic";

export async function GET() {
  return backendResponse(await callBackend("/v1/servers", {}));
}

export async function POST(request: NextRequest) {
  if (!sameOriginMutation(request)) {
    return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  }
  const body = await requestJSON(request);
  if (!body || typeof body !== "object") {
    return errorResponse(400, "invalid_request", "A server name and optional hostname are required.");
  }
  return backendResponse(await callBackend("/v1/servers", { method: "POST", body }));
}
