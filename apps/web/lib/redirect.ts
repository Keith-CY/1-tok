import { NextResponse } from "next/server";

const redirectOrigin = "http://portal.internal";

export function redirectToPath(path: string): NextResponse {
	const nextURL = new URL(path, redirectOrigin);
	return new NextResponse(null, {
		status: 303,
		headers: {
			Location: `${nextURL.pathname}${nextURL.search}${nextURL.hash}`,
		},
	});
}
