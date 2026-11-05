import sharp from "sharp";

/** Re-encodes a PNG as a quantized palette image, much smaller and visually the same for UI. */
export async function optimizePng(input: Buffer): Promise<Buffer> {
  return sharp(input).png({ palette: true, quality: 90, effort: 10, compressionLevel: 9 }).toBuffer();
}
