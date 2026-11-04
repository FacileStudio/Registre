/**
 * The browser half of a passkey ceremony. Registre sends WebAuthn options as
 * JSON with binary fields in base64url; navigator.credentials wants
 * ArrayBuffers, and answers with a PublicKeyCredential the server wants back
 * as JSON. These functions are the two conversions, and nothing else.
 */

type Descriptor = { id: string; type: PublicKeyCredentialType; transports?: AuthenticatorTransport[] };

/** decode turns base64url into bytes. */
function decode(value: string): ArrayBuffer {
	const base64 = value.replace(/-/g, '+').replace(/_/g, '/');
	const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4);
	const binary = atob(padded);
	const bytes = new Uint8Array(binary.length);
	for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
	return bytes.buffer;
}

/** encode turns bytes into base64url without padding. */
function encode(value: ArrayBuffer | null): string {
	if (!value) return '';
	let binary = '';
	for (const byte of new Uint8Array(value)) binary += String.fromCharCode(byte);
	return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

/** descriptors decodes a list of credential descriptors. */
function descriptors(list: Descriptor[] | undefined): PublicKeyCredentialDescriptor[] | undefined {
	return list?.map((item) => ({ ...item, id: decode(item.id) }));
}

/** passkeysSupported reports whether this browser can run a ceremony at all. */
export function passkeysSupported(): boolean {
	return typeof window !== 'undefined' && 'PublicKeyCredential' in window && !!navigator.credentials;
}

/** createPasskey runs a registration and answers the credential as JSON. */
export async function createPasskey(publicKey: Record<string, any>): Promise<Record<string, unknown>> {
	const options: PublicKeyCredentialCreationOptions = {
		...(publicKey as PublicKeyCredentialCreationOptions),
		challenge: decode(publicKey.challenge),
		user: { ...publicKey.user, id: decode(publicKey.user.id) },
		excludeCredentials: descriptors(publicKey.excludeCredentials)
	};
	const credential = (await navigator.credentials.create({ publicKey: options })) as PublicKeyCredential;
	const response = credential.response as AuthenticatorAttestationResponse;
	return {
		id: credential.id,
		rawId: encode(credential.rawId),
		type: credential.type,
		authenticatorAttachment: credential.authenticatorAttachment ?? undefined,
		clientExtensionResults: credential.getClientExtensionResults(),
		response: {
			clientDataJSON: encode(response.clientDataJSON),
			attestationObject: encode(response.attestationObject),
			transports: response.getTransports?.() ?? []
		}
	};
}

/** getPasskey runs an assertion and answers the credential as JSON. */
export async function getPasskey(publicKey: Record<string, any>): Promise<Record<string, unknown>> {
	const options: PublicKeyCredentialRequestOptions = {
		...(publicKey as PublicKeyCredentialRequestOptions),
		challenge: decode(publicKey.challenge),
		allowCredentials: descriptors(publicKey.allowCredentials)
	};
	const credential = (await navigator.credentials.get({ publicKey: options })) as PublicKeyCredential;
	const response = credential.response as AuthenticatorAssertionResponse;
	return {
		id: credential.id,
		rawId: encode(credential.rawId),
		type: credential.type,
		authenticatorAttachment: credential.authenticatorAttachment ?? undefined,
		clientExtensionResults: credential.getClientExtensionResults(),
		response: {
			clientDataJSON: encode(response.clientDataJSON),
			authenticatorData: encode(response.authenticatorData),
			signature: encode(response.signature),
			userHandle: encode(response.userHandle)
		}
	};
}
