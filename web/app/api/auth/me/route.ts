import { NextRequest, NextResponse } from "next/server";
import {
  backendResponse,
  callBackend,
  clearSessionCookie,
  errorResponse,
  sessionToken,
} from "@/lib/server-api";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  const token = sessionToken(request);
  if (!token) return errorResponse(401, "unauthorized", "Sign in to continue.");

  const result = await callBackend("/v1/auth/me", { token });
  const response = backendResponse(result);
  if (result.status === 401) clearSessionCookie(response, request);
  return response;
}
