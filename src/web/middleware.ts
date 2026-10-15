import { NextResponse, type NextRequest } from 'next/server'

/**
 * the route guard.
 * ! this checks that a cookie exists. that is all it can do and all it is for — the sessions
 * ! table lives in go and RequireAuth re-reads it on every single request, which is the only
 * ! thing actually protecting anything here. treating this file as authorization is the classic
 * ! mistake in this stack, and it is why web/ owns no state in the first place
 * ? so it is a UX guard: it saves a reader from watching a page render and then bounce
 */
const PUBLIC_PATHS = ['/login', '/register']

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl
  //? the cookie name is session.CookieName on the go side. a value is never validated here,
  //? only its presence, because validating it would mean reading the database
  const hasSession = Boolean(request.cookies.get('sid')?.value)
  const isPublic = PUBLIC_PATHS.includes(pathname)

  if (!hasSession && !isPublic) {
    const to = request.nextUrl.clone()
    to.pathname = '/login'
    //? remember where they were going, so the login can send them back rather than always
    //? dumping everyone on the feed
    if (pathname !== '/') to.searchParams.set('next', pathname)
    return NextResponse.redirect(to)
  }

  if (hasSession && isPublic) {
    const to = request.nextUrl.clone()
    to.pathname = '/feed'
    to.search = ''
    return NextResponse.redirect(to)
  }

  return NextResponse.next()
}

export const config = {
  /**
   * everything except next's own assets and the files in /public.
   * ? lain.gif and the favicon have to stay reachable without a session, or the login page
   * ? loads without its own furniture
   */
  matcher: ['/((?!_next/static|_next/image|favicon.ico|.*\\.(?:gif|png|jpg|jpeg|svg|ico|webp)$).*)'],
}
