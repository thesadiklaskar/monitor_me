import { NextRequest } from "next/server";
import {
  backendResponse,
  callBackend,
  errorResponse,
  sameOriginMutation,
  sessionToken,
} from "@/lib/server-api";

export const dynamic = "force-dynamic";

export async function POST(request: NextRequest) {
  if (!sameOriginMutation(request)) {
    return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  }
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to start a live connection.");
  return backendResponse(
    await callBackend("/v1/live/ticket", { method: "POST", token, body: {} }),
  );
}
