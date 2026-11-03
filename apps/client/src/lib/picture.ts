/** PICTURE_SIDE is the edge, in pixels, of the square the browser uploads. */
export const PICTURE_SIDE = 256;

/**
 * squarePicture draws the centred square of an image file onto a canvas of
 * PICTURE_SIDE and returns it as a PNG.
 *
 * The browser does the crop so a phone photo uploads as tens of kilobytes
 * rather than megabytes, and so HEIC or WebP, which registre does not decode,
 * still arrive as something it does. registre crops and re-encodes again on
 * its side; this is the convenience, not the trust boundary.
 */
export async function squarePicture(file: File): Promise<Blob> {
	const bitmap = await createImageBitmap(file);
	const side = Math.min(bitmap.width, bitmap.height);
	const canvas = document.createElement('canvas');
	canvas.width = PICTURE_SIDE;
	canvas.height = PICTURE_SIDE;
	const context = canvas.getContext('2d');
	if (!context) throw new Error('This browser cannot prepare the picture.');
	context.imageSmoothingQuality = 'high';
	context.drawImage(
		bitmap,
		(bitmap.width - side) / 2,
		(bitmap.height - side) / 2,
		side,
		side,
		0,
		0,
		PICTURE_SIDE,
		PICTURE_SIDE
	);
	bitmap.close();
	return new Promise((resolve, reject) =>
		canvas.toBlob(
			(blob) => (blob ? resolve(blob) : reject(new Error('The picture could not be encoded.'))),
			'image/png'
		)
	);
}
