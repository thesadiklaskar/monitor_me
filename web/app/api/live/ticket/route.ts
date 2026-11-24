import { NextRequest } from "next/server";
import {
  backendResponse,
  callBackend,
  errorResponse,
  sameOriginMutation,
} from "@/lib/server-api";

export const dynamic = "force-dynamic";

export async function POST(request: NextRequest) {
  if (!sameOriginMutation(request)) {
    return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  }
  return backendResponse(
    await callBackend("/v1/live/ticket", { method: "POST", body: {} }),
  );
}
