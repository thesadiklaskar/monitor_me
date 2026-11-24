import { NextRequest } from "next/server";
import {
  backendResponse,
  callBackend,
  errorResponse,
  sameOriginMutation,
} from "@/lib/server-api";

export const dynamic = "force-dynamic";

type RouteContext = { params: Promise<{ tokenId: string }> };

export async function POST(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) {
    return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  }
  const { tokenId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(tokenId)) {
    return errorResponse(400, "invalid_token_id", "The enrollment-token identifier is invalid.");
  }
  return backendResponse(
    await callBackend(`/v1/enrollment-tokens/${encodeURIComponent(tokenId)}/revoke`, {
      method: "POST", body: {},
    }),
  );
}
