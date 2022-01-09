export function Base64EncodeFile(file): Promise<string>{
    return new Promise((resolve, reject) => {
        const reader = new FileReader();
        reader.readAsDataURL(file);
        reader.onload = () => {
            resolve(reader.result.toString());
        };
        reader.onerror = (error) => {
            reject(error);
        };
    });
} 