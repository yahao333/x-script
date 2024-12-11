from PIL import Image
from winsdk.windows.graphics.imaging import SoftwareBitmap, BitmapAlphaMode, BitmapPixelFormat
import numpy as np
import asyncio
import winsdk.windows.media.ocr as ocr
import winsdk.windows.storage as storage
import winsdk.windows.graphics.imaging as imaging
import winsdk.windows.globalization as globalization
import logging
import os

# 配置日志
logging.basicConfig(
    level=logging.DEBUG,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

def scale_image_uniform(image_path, scale_factor):
    """Scale the image uniformly by the given factor."""
    try:
        with Image.open(image_path) as img:
            new_width = int(img.width * scale_factor)
            new_height = int(img.height * scale_factor)
            return img.resize((new_width, new_height), Image.Resampling.LANCZOS)
    except Exception as e:
        logger.error(f"Error scaling image: {str(e)}")
        return None
    
async def perform_ocr(image_path):
    try:
        logger.debug(f"Starting OCR process for image: {image_path}")
        
        temp_file_path = None
        random_access_stream = None

        # Check if scaling is needed and process image
        with Image.open(image_path) as img:
            # Assuming OcrEngine.MaxImageDimension is around 2048 pixels
            MAX_IMAGE_DIMENSION = ocr.OcrEngine.max_image_dimension
            should_scale = img.width * 1.5 <= MAX_IMAGE_DIMENSION
            scale_factor = 1.5 if should_scale else 1.0
            
            logger.debug(f"Image dimensions: {img.width}x{img.height}")
            logger.debug(f"Scaling factor: {scale_factor}")
            
            # Scale image if needed
            scaled_img = scale_image_uniform(image_path, scale_factor)
            if scaled_img is None:
                return None
            logger.debug(f"Scaled image dimensions: {scaled_img.width}x{scaled_img.height}")
            # Save scaled image to temporary memory stream
            from io import BytesIO
            memory_stream = BytesIO()
            scaled_img.save(memory_stream, format='BMP')
            memory_stream.seek(0)
            
            logger.debug("Saving scaled image to temporary memory stream")

            # Create RandomAccessStream from memory stream
            from winsdk.windows.storage.streams import Buffer, InMemoryRandomAccessStream, DataWriter
            
            # 创建 InMemoryRandomAccessStream
            random_access_stream = InMemoryRandomAccessStream()
            
            # 使用 DataWriter 写入数据
            writer = DataWriter(random_access_stream)
            bytes_data = memory_stream.getvalue()
            writer.write_bytes(bytes_data)
            await writer.store_async()
            # writer.close()
            
            # 将流指针重置到开始位置
            random_access_stream.seek(0)

            logger.debug("Created stream from memory directly")

            # Create StorageFile from memory stream
            import tempfile
            with tempfile.NamedTemporaryFile(delete=False, suffix='.bmp') as temp_file:
                temp_file_path = temp_file.name
                # 写入缓存数据
                temp_file.write(memory_stream.getvalue())

            logger.debug(f"Temporary file created at: {temp_file_path}")    
        
        # 使用 get_file_from_path_async 替代直接操作 StorageFile
        # temp_storage_file = await storage.StorageFile.get_file_from_path_async(temp_file_path)

        # # Open file stream for the temporary file
        # stream = await temp_storage_file.open_async(storage.FileAccessMode.READ)
        stream = random_access_stream
        logger.debug("File stream opened successfully")
        
        # Create BitmapDecoder
        logger.debug("Creating BitmapDecoder...")
        decoder = await imaging.BitmapDecoder.create_async(stream)
        logger.debug("BitmapDecoder created successfully")
        
        # Get soft bitmap
        logger.debug("Getting SoftBitmap...")
        bitmap = await decoder.get_software_bitmap_async()
        logger.debug(f"SoftBitmap obtained: Format={bitmap.bitmap_pixel_format}, AlphaMode={bitmap.bitmap_alpha_mode}")
        
        # Create OCR engine
        logger.debug("Creating OCR engine...")
        language = globalization.Language("zh-Hans")
        engine = ocr.OcrEngine.try_create_from_language(language)
        
        if not engine:
            logger.warning("Could not create OCR engine from user profile, falling back to English")
            engine = ocr.OcrEngine.try_create("en")
            
        if engine:
            logger.debug("OCR engine created successfully")
            logger.debug(f"OCR engine language: {engine.recognizer_language.display_name}")
        else:
            logger.error("Failed to create OCR engine")
            return None
        
        # Perform OCR recognition
        logger.debug("Starting OCR recognition...")
        result = await engine.recognize_async(bitmap)
        logger.debug("OCR recognition completed")
        
        # 清理临时文件
        try:
            os.unlink(temp_file_path)
            logger.debug("Temporary file deleted successfully")
        except Exception as e:
            logger.warning(f"Failed to delete temporary file: {str(e)}")    

        # Print recognized text
        if result:
            logger.info("Text recognition successful")
            logger.debug(f"Recognized text: {result.text}")
            
            # Detailed line and word information
            for i, line in enumerate(result.lines, 1):
                logger.debug(f"Line {i}: {line.text}")
                for j, word in enumerate(line.words, 1):
                    logger.debug(f"  Word {j}: {word.text}")
            
            return result.text
        else:
            logger.warning("No text was recognized")
            return None
    
    except Exception as e:
        logger.error(f"OCR Error: {str(e)}", exc_info=True)
        return None
    
async def perform_ocr_v1(image_path):
    try:
        logger.debug(f"Starting OCR process for image: {image_path}")
        
        # Get StorageFile from path
        logger.debug("Getting StorageFile from path...")
        file = await storage.StorageFile.get_file_from_path_async(image_path)
        logger.debug(f"Successfully got StorageFile: {file.path}")
        
        # Open file stream
        logger.debug("Opening file stream...")
        stream = await file.open_async(storage.FileAccessMode.READ)
        logger.debug("File stream opened successfully")
        
        # Create BitmapDecoder
        logger.debug("Creating BitmapDecoder...")
        decoder = await imaging.BitmapDecoder.create_async(stream)
        logger.debug("BitmapDecoder created successfully")
        
        # Get soft bitmap
        logger.debug("Getting SoftBitmap...")
        bitmap = await decoder.get_software_bitmap_async()
        logger.debug(f"SoftBitmap obtained: Format={bitmap.bitmap_pixel_format}, AlphaMode={bitmap.bitmap_alpha_mode}")
        
        # Create OCR engine
        logger.debug("Creating OCR engine...")
        language = globalization.Language("zh-Hans")
        engine = ocr.OcrEngine.try_create_from_language(language)
        
        if not engine:
            logger.warning("Could not create OCR engine from user profile, falling back to English")
            engine = ocr.OcrEngine.try_create("en")
            
        if engine:
            logger.debug("OCR engine created successfully")
            logger.debug(f"OCR engine language: {engine.recognizer_language.display_name}")
        else:
            logger.error("Failed to create OCR engine")
            return None
        
        # Perform OCR recognition
        logger.debug("Starting OCR recognition...")
        result = await engine.recognize_async(bitmap)
        logger.debug("OCR recognition completed")
        
        # Print recognized text
        if result:
            logger.info("Text recognition successful")
            logger.debug(f"Recognized text: {result.text}")
            
            # Detailed line and word information
            for i, line in enumerate(result.lines, 1):
                logger.debug(f"Line {i}: {line.text}")
                for j, word in enumerate(line.words, 1):
                    logger.debug(f"  Word {j}: {word.text}")
            
            return result.text
        else:
            logger.warning("No text was recognized")
            return None
    
    except Exception as e:
        logger.error(f"OCR Error: {str(e)}", exc_info=True)
        return None
    
def convert_png_to_bmp(png_path, bmp_path=None):
    """Convert a PNG image to BMP format.
    
    Args:
        png_path: Path to source PNG file
        bmp_path: Optional path for output BMP file. If None, will use same name as PNG but with .bmp extension
        
    Returns:
        Path to the converted BMP file on success, None on failure
    """
    try:
        # Validate input file exists
        if not os.path.exists(png_path):
            logger.error(f"Input file does not exist: {png_path}")
            return None
            
        # Generate output path if not provided
        if bmp_path is None:
            bmp_path = os.path.splitext(png_path)[0] + '.bmp'
            
        logger.debug(f"Converting {png_path} to BMP format")
        
        # Open and convert image
        with Image.open(png_path) as img:
            # Convert to RGB mode to remove alpha channel if present
            if img.mode in ('RGBA', 'LA') or (img.mode == 'P' and 'transparency' in img.info):
                background = Image.new('RGB', img.size, (255, 255, 255))
                if img.mode == 'P':
                    img = img.convert('RGBA')
                background.paste(img, mask=img.split()[3])  # Use alpha channel as mask
                img = background
            elif img.mode != 'RGB':
                img = img.convert('RGB')
                
            # Save as BMP
            img.save(bmp_path, 'BMP')
            
        logger.debug(f"Successfully converted to: {bmp_path}")
        return bmp_path
        
    except Exception as e:
        logger.error(f"Error converting PNG to BMP: {str(e)}")
        return None
    
if __name__ == "__main__":
    logger.info("Starting main program")
    try:
        bmp_file = convert_png_to_bmp("screenshot02.png")
        result = asyncio.run(perform_ocr(bmp_file))
        if result:
            logger.info("OCR completed successfully")
            logger.info(f"Final result: {result}")
        else:
            logger.error("OCR failed to produce results")
    except Exception as e:
        logger.error(f"Main program error: {str(e)}", exc_info=True)
