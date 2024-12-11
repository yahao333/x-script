import asyncio
from PIL import Image
import winrt.windows.media.ocr as ocr
from winsdk.windows.graphics.imaging import BitmapDecoder
from winsdk.windows.storage.streams import InMemoryRandomAccessStream, DataWriter
import io
import os

async def ocr_image_async(image_path):
    """Asynchronously perform OCR on an image using Windows.Media.OCR"""
    # Load image using PIL
    pil_image = Image.open(image_path)
    
    # Specify a language for the OCR engine
    language = ocr.OcrEngine.available_recognizer_languages[0]  # Use the first available language
    engine = ocr.OcrEngine.try_create_from_language(language)
    
    # Convert PIL image to bytes
    img_byte_arr = io.BytesIO()
    pil_image.save(img_byte_arr, format='PNG')
    img_byte_arr = img_byte_arr.getvalue()

    # Create stream
    stream = InMemoryRandomAccessStream()
    writer = DataWriter(stream)
    writer.write_bytes(img_byte_arr)
    await writer.store_async()
    stream.seek(0)

    # Create bitmap
    decoder = await BitmapDecoder.create_async(stream)
    bitmap = await decoder.get_software_bitmap_async()

    # Perform OCR
    result = await engine.recognize_async(bitmap)
    
    # Extract text from result
    text = "\n".join([line.text for line in result.lines])
    return text

def ocr_image(image_path):
    """Synchronous wrapper for OCR function"""
    return asyncio.run(ocr_image_async(image_path))

if __name__ == "__main__":
    # Test image path
    image_path = "test.png"
    
    if os.path.exists(image_path):
        recognized_text = ocr_image(image_path)
        print("识别的文本内容：")
        print(recognized_text)
    else:
        print("指定的图像文件不存在。")