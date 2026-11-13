/**
 * Rules the server enforces and the pages check early, so a person is told
 * before they submit rather than after.
 *
 * These are a courtesy, never the enforcement. The server refuses on its own
 * terms whatever this file says, which is the only reason it is safe for the
 * browser to hold a copy at all.
 */

/**
 * MIN_PASSWORD mirrors credentials.MinPasswordLength in the API.
 *
 * It is one constant here for the same reason it became one constant there:
 * it was written out separately on the account page, the invitation page and
 * the reset page, and three numbers that agree today are three numbers that
 * can disagree tomorrow. The reset path was the one that had drifted low on
 * the server, which made the emailed link the way around the stricter form.
 */
export const MIN_PASSWORD = 8;

/** MAX_PASSWORD mirrors credentials.MaxPasswordLength in the API. */
export const MAX_PASSWORD = 128;
