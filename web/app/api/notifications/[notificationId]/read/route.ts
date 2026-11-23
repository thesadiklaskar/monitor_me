import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, sameOriginMutation, sessionToken } from "@/lib/server-api";
export const dynamic = "force-dynamic";
type RouteContext = { params: Promise<{ notificationId: string }> };
export async function POST(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to update a notification.");
  const { notificationId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(notificationId)) return errorResponse(400, "invalid_notification_id", "The notification identifier is invalid.");
  return backendResponse(await callBackend("/v1/notifications/" + encodeURIComponent(notificationId) + "/read", { method: "POST", token, body: {} }));
}
