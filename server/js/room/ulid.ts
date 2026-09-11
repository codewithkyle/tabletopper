


















const CROCKFORD = "0123456789ABCDEFGHJKMNPQRSTVWXYZ";




const TIME_CHARS = 10;
const RANDOM_BYTES = 10;











let lastTime = 0;
const lastRandom = new Uint8Array(RANDOM_BYTES);

export function ulid(now: number = Date.now()): string {
	const time = Math.max(0, Math.floor(now));

	if (time === lastTime) {
		increment(lastRandom);
	} else {
		lastTime = time;
		crypto.getRandomValues(lastRandom);
	}

	return encodeTime(time) + encodeRandom(lastRandom);
}






function encodeTime(time: number): string {
	let out = "";
	let left = time;

	for (let i = 0; i < TIME_CHARS; i++) {
		out = CROCKFORD[left % 32] + out;
		left = Math.floor(left / 32);
	}

	return out;
}



function encodeRandom(bytes: Uint8Array): string {
	let out = "";
	let value = 0;
	let bits = 0;

	for (const byte of bytes) {
		
		
		
		value = (value << 8) | byte;
		bits += 8;

		while (bits >= 5) {
			bits -= 5;
			out += CROCKFORD[(value >>> bits) & 31];
		}
	}

	return out;
}




function increment(bytes: Uint8Array): void {
	for (let i = bytes.length - 1; i >= 0; i--) {
		if (bytes[i] < 255) {
			bytes[i]++;

			return;
		}

		bytes[i] = 0;
	}
}
