import { NextRequest } from "next/server";
import { updateSession } from "./lib/session/session"

export async function middleware(request: NextRequest) {
    return await updateSession(request);
}