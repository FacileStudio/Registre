/**
 * Solving an ALTCHA proof-of-work challenge in the browser.
 *
 * The server signs a challenge naming a salt and the SHA-256 of salt plus a
 * secret number below maxNumber; finding the number costs the browser about
 * half a second and costs a script sending a thousand sign-ups a thousand
 * times that. The answer is the base64 JSON payload altcha-lib-go verifies.
 * No third party is involved: the challenge comes from Registre itself.
 */

/** Challenge is what GET /api/signup/challenge answers. */
export interface Challenge {
	algorithm: string;
	challenge: string;
	maxNumber: number;
	salt: string;
	signature: string;
}

/** hex digests text with SHA-256. */
async function hex(text: string): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text));
	return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('');
}

/** solve finds the number and answers the payload, or throws when none fits. */
export async function solve(challenge: Challenge): Promise<string> {
	for (let number = 0; number <= challenge.maxNumber; number++) {
		if ((await hex(challenge.salt + number)) === challenge.challenge) {
			return btoa(
				JSON.stringify({
					algorithm: challenge.algorithm,
					challenge: challenge.challenge,
					number,
					salt: challenge.salt,
					signature: challenge.signature
				})
			);
		}
	}
	throw new Error('altcha: no solution');
}

/** fetchAndSolve gets a fresh challenge and solves it. */
export async function fetchAndSolve(): Promise<string> {
	const response = await fetch('/api/signup/challenge', { cache: 'no-store' });
	if (!response.ok) throw new Error(`altcha: challenge answered ${response.status}`);
	return solve((await response.json()) as Challenge);
}
