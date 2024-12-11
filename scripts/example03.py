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

import win32gui
import win32ui
import win32con
import win32process
import win32gui
import win32api
import time
from ctypes import windll, Structure, c_long, byref
import threading

# 配置日志
logging.basicConfig(
    level=logging.DEBUG,
    format='%(asctime)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

class POINT(Structure):
    _fields_ = [("x", c_long), ("y", c_long)]

def get_real_window_rect(hwnd):
    """
    获取窗口的真实位置（考虑DPI缩放）
    """
    try:
        # 获取窗口DPI缩放因子
        try:
            from ctypes.wintypes import HANDLE, RECT
            from ctypes import POINTER, c_int
            
            # 定义GetDpiForWindow函数
            GetDpiForWindow = windll.user32.GetDpiForWindow
            GetDpiForWindow.argtypes = [HANDLE]
            GetDpiForWindow.restype = c_int
            
            # 获取窗口DPI
            dpi = GetDpiForWindow(hwnd)
            scale_factor = dpi / 96.0  # 96 is the default DPI
            logger.debug(f"窗口DPI: {dpi}, 缩放因子: {scale_factor}")
        except Exception as e:
            logger.warning(f"获取DPI失败，使用默认缩放: {e}")
            scale_factor = 1.0

        # 获取窗口位置
        rect = win32gui.GetWindowRect(hwnd)
        left, top, right, bottom = rect
        logger.debug(f"原始窗口位置: left={left}, top={top}, right={right}, bottom={bottom}")
        
        # 应用DPI缩放
        left = int(left * scale_factor)
        top = int(top * scale_factor)
        right = int(right * scale_factor)
        bottom = int(bottom * scale_factor)
        
        logger.debug(f"调整后窗口位置: left={left}, top={top}, right={right}, bottom={bottom}")
        return left, top, right, bottom
        
    except Exception as e:
        logger.error(f"获取真实窗口位置失败: {e}")
        return win32gui.GetWindowRect(hwnd)
    
def draw_highlight_border(hwnd, flash_count=3, flash_delay=0.2):
    """
    在选中的窗口周围绘制闪烁边框
    
    Args:
        hwnd: 窗口句柄
        flash_count: 闪烁次数
        flash_delay: 闪烁间隔(秒)
    """
    borders = []
    try:
        logger.debug(f"开始绘制窗口边框高亮，窗口句柄: {hwnd}")
        
        # 获取窗口位置和尺寸
        left, top, right, bottom = get_real_window_rect(hwnd)
        width = right - left
        height = bottom - top
        logger.debug(f"窗口尺寸: width={width}, height={height}")
        
        # 创建临时窗口类
        wc = win32gui.WNDCLASS()
        wc.lpszClassName = "BorderWindow"
        wc.hbrBackground = win32gui.GetStockObject(win32con.NULL_BRUSH)
        wc.style = win32con.CS_HREDRAW | win32con.CS_VREDRAW
        wc.lpfnWndProc = win32gui.DefWindowProc
        wc.hCursor = win32gui.LoadCursor(0, win32con.IDC_ARROW)
        
        try:
            win32gui.RegisterClass(wc)
        except Exception:
            pass
        
        # 创建四个边框窗口
        styles = win32con.WS_POPUP | win32con.WS_VISIBLE
        ex_styles = win32con.WS_EX_TOOLWINDOW | win32con.WS_EX_TOPMOST | win32con.WS_EX_TRANSPARENT | win32con.WS_EX_LAYERED

        border_thickness = 3
        border_positions = [
            (left, top, width, border_thickness),                    # 上
            (left, bottom - border_thickness, width, border_thickness), # 下
            (left, top, border_thickness, height),                   # 左
            (right - border_thickness, top, border_thickness, height)  # 右
        ]

        for x, y, w, h in border_positions:
            hwnd_border = win32gui.CreateWindowEx(
                ex_styles, "BorderWindow", None, styles,
                x, y, w, h, 0, 0, 0, None
            )
            borders.append(hwnd_border)

        # 闪烁动画
        colors = [(255, 0, 0), (255, 255, 0)]  # 红色和黄色交替
        for _ in range(flash_count):
            for color in colors:
                color_rgb = win32api.RGB(*color)
                for border in borders:
                    win32gui.SetLayeredWindowAttributes(
                        border, color_rgb, 180, 
                        win32con.LWA_ALPHA | win32con.LWA_COLORKEY
                    )
                logger.debug(f"边框颜色设置为: {color}")
                time.sleep(flash_delay)

    except Exception as e:
        logger.error(f"绘制高亮边框失败: {str(e)}")
        logger.exception("详细错误信息:")
    finally:
        # 清理边框窗口
        for border in borders:
            try:
                win32gui.DestroyWindow(border)
            except Exception:
                pass

def get_window_at_cursor():
    """
    获取鼠标指针下的窗口句柄
    """
    point = POINT()
    windll.user32.GetCursorPos(byref(point))
    return win32gui.WindowFromPoint((point.x, point.y))

def capture_window(hwnd):
    """
    捕获指定窗口的截图
    
    Args:
        hwnd: 窗口句柄
    
    Returns:
        PIL.Image 对象或 None（如果失败）
    """
    try:
        # 获取窗口尺寸
        # left, top, right, bottom = win32gui.GetWindowRect(hwnd)
        left, top, right, bottom = get_real_window_rect(hwnd)
        width = right - left
        height = bottom - top
        logger.debug(f"截屏窗口尺寸: width={width}, height={height}")
        
        # 创建设备上下文
        hwndDC = win32gui.GetWindowDC(hwnd)
        mfcDC = win32ui.CreateDCFromHandle(hwndDC)
        saveDC = mfcDC.CreateCompatibleDC()
        
        # 创建位图对象
        saveBitMap = win32ui.CreateBitmap()
        saveBitMap.CreateCompatibleBitmap(mfcDC, width, height)
        saveDC.SelectObject(saveBitMap)
        
        # 复制窗口内容到位图
        result = windll.user32.PrintWindow(hwnd, saveDC.GetSafeHdc(), 3)
        
        if result != 1:
            logger.error("PrintWindow failed")
            return None
        
        # 转换为PIL Image
        bmpinfo = saveBitMap.GetInfo()
        bmpstr = saveBitMap.GetBitmapBits(True)
        
        from PIL import Image
        image = Image.frombuffer(
            'RGB',
            (bmpinfo['bmWidth'], bmpinfo['bmHeight']),
            bmpstr, 'raw', 'BGRX', 0, 1)
        
        # 清理资源
        win32gui.DeleteObject(saveBitMap.GetHandle())
        saveDC.DeleteDC()
        mfcDC.DeleteDC()
        win32gui.ReleaseDC(hwnd, hwndDC)
        
        return image
        
    except Exception as e:
        logger.error(f"Error capturing window: {str(e)}")
        return None

def select_window_for_capture():
    """
    交互式窗口选择器
    
    Returns:
        (hwnd, PIL.Image) 元组或 (None, None)
    """
    try:
        print("请将鼠标移动到要截图的窗口上，然后按住左键拖动...")
        print("按 Esc 键取消")
        
        last_hwnd = None
        
        # 等待左键按下
        while True:
            if win32api.GetAsyncKeyState(win32con.VK_ESCAPE):
                print("已取消选择")
                return None, None
                
            # 获取当前鼠标下的窗口
            current_hwnd = get_window_at_cursor()
            
            # 如果鼠标移动到了新窗口上，更新高亮
            if current_hwnd != last_hwnd and current_hwnd:
                last_hwnd = current_hwnd
                # 获取窗口标题
                window_title = win32gui.GetWindowText(current_hwnd)
                if window_title:  # 只显示有标题的窗口
                    print(f"当前窗口: {window_title}")
                
            if win32api.GetAsyncKeyState(win32con.VK_LBUTTON) & 0x8000:
                # 获取鼠标下的窗口
                hwnd = get_window_at_cursor()
                if hwnd:
                    # 高亮显示选中的窗口
                    draw_highlight_border(hwnd)
                    win32gui.SetForegroundWindow(hwnd)
                    
                    # 等待左键释放
                    while win32api.GetAsyncKeyState(win32con.VK_LBUTTON) & 0x8000:
                        time.sleep(0.1)
                    
                    # 获取窗口标题和PID
                    window_title = win32gui.GetWindowText(hwnd)
                    thread_id, pid = win32process.GetWindowThreadProcessId(hwnd)
                    
                    # 添加更详细的进程信息输出
                    try:
                        import psutil
                        process = psutil.Process(pid)
                        print(f"""
窗口信息:
- 标题: {window_title}
- PID: {pid}
- 进程名称: {process.name()}
- 执行文件路径: {process.exe()}
- 父进程 PID: {process.ppid()}
- 状态: {process.status()}
- 进程ID: {thread_id}
""")
                    except psutil.NoSuchProcess:
                        print(f"进程 {pid} 已经结束")
                    except psutil.AccessDenied:
                        print(f"无权限访问进程 {pid} 的详细信息")
                    
                    # 捕获窗口截图
                    image = capture_window(hwnd)
                    return hwnd, image
            
            time.sleep(0.1)
            
    except Exception as e:
        logger.error(f"Error selecting window: {str(e)}")
        return None, None
        
def capture_window_by_pid(target_pid):
    """
    通过PID查找窗口并截图
    
    Args:
        target_pid: 进程ID
    
    Returns:
        PIL.Image 对象或 None（如果失败）
    """
    def callback(hwnd, hwnds):
        if win32gui.IsWindowVisible(hwnd):
            _, pid = win32process.GetWindowThreadProcessId(hwnd)
            if pid == target_pid:
                hwnds.append(hwnd)
        return True

    try:
        # 验证PID是否存在
        try:
            import psutil
            process = psutil.Process(target_pid)
            logger.info(f"""找到目标进程:
- PID: {target_pid}
- 进程名称: {process.name()}
- 执行路径: {process.exe()}
- 状态: {process.status()}""")
        except psutil.NoSuchProcess:
            logger.error(f"PID {target_pid} 不存在")
            return None
        except psutil.AccessDenied:
            logger.warning(f"无权限访问进程 {target_pid} 的详细信息")

        # 查找进程的所有可见窗口
        hwnds = []
        win32gui.EnumWindows(callback, hwnds)
        
        if not hwnds:
            logger.warning(f"未找到PID {target_pid} 对应的可见窗口")
            return None
            
        # 获取第一个匹配的窗口
        hwnd = hwnds[0]
        window_title = win32gui.GetWindowText(hwnd)
        logger.info(f"找到窗口: {window_title}")
        
        # 捕获窗口截图
        image = capture_window(hwnd)
        
        if image:
            logger.info(f"成功捕获PID {target_pid} 的窗口截图")
            return image
        else:
            logger.error("截图失败")
            return None
        
    except Exception as e:
        logger.error(f"捕获窗口失败: {str(e)}")
        return None
            
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
    


def capture_window_by_process_name(process_name):
    """
    通过进程名称查找窗口并截图
    
    Args:
        process_name: 进程名称（例如 'notepad.exe'）
    
    Returns:
        PIL.Image 对象或 None（如果失败）
    """
    def callback(hwnd, hwnds):
        if win32gui.IsWindowVisible(hwnd):
            _, pid = win32process.GetWindowThreadProcessId(hwnd)
            try:
                # 获取进程名称
                import psutil
                process = psutil.Process(pid)
                if process.name().lower() == process_name.lower():
                    hwnds.append(hwnd)
            except (psutil.NoSuchProcess, psutil.AccessDenied):
                pass
        return True

    try:
        hwnds = []
        win32gui.EnumWindows(callback, hwnds)
        
        if not hwnds:
            logger.warning(f"No visible windows found for process: {process_name}")
            return None
            
        # 获取第一个匹配的窗口
        hwnd = hwnds[0]
        
        # 获取窗口尺寸
        left, top, right, bottom = win32gui.GetWindowRect(hwnd)
        width = right - left
        height = bottom - top
        
        # 创建设备上下文
        hwndDC = win32gui.GetWindowDC(hwnd)
        mfcDC = win32ui.CreateDCFromHandle(hwndDC)
        saveDC = mfcDC.CreateCompatibleDC()
        
        # 创建位图对象
        saveBitMap = win32ui.CreateBitmap()
        saveBitMap.CreateCompatibleBitmap(mfcDC, width, height)
        saveDC.SelectObject(saveBitMap)
        
        # 复制窗口内容到位图
        result = windll.user32.PrintWindow(hwnd, saveDC.GetSafeHdc(), 3)
        
        if result != 1:
            logger.error("PrintWindow failed")
            return None
        
        # 转换为PIL Image
        bmpinfo = saveBitMap.GetInfo()
        bmpstr = saveBitMap.GetBitmapBits(True)
        
        from PIL import Image
        image = Image.frombuffer(
            'RGB',
            (bmpinfo['bmWidth'], bmpinfo['bmHeight']),
            bmpstr, 'raw', 'BGRX', 0, 1)
        
        # 清理资源
        win32gui.DeleteObject(saveBitMap.GetHandle())
        saveDC.DeleteDC()
        mfcDC.DeleteDC()
        win32gui.ReleaseDC(hwnd, hwndDC)
        
        logger.debug(f"Successfully captured window for process: {process_name}")
        return image
        
    except Exception as e:
        logger.error(f"Error capturing window: {str(e)}")
        return None

if __name__ == "__main__":
    logger.info("Starting main program")
    try:
        # 选择要截图的窗口
        hwnd, image = select_window_for_capture()

        for i in range(10):
            # 捕获窗口截图
            image = capture_window(hwnd)
            if image:
                # 保存截图
                image.save(f"screenshot_{i}.bmp")
            time.sleep(3)
        
        # if image:
        #     # 保存截图
        #     image.save("screenshot.bmp")
        #     # # 执行OCR
        #     # result = asyncio.run(perform_ocr("screenshot.bmp"))
        #     # if result:
        #     #     logger.info("OCR completed successfully")
        #     #     logger.info(f"Final result: {result}")
        #     # else:
        #     #     logger.error("OCR failed to produce results")
        # else:
        #     logger.error("Failed to capture window")
    except Exception as e:
        logger.error(f"Main program error: {str(e)}", exc_info=True)

# if __name__ == "__main__":
#     logger.info("Starting main program")
#     try:
#         # 示例：通过PID捕获窗口
#         target_pid = 12576  # 替换为你要截图的进程PID
#         image = capture_window_by_pid(target_pid)
        
#         if image:
#             # 保存截图
#             image.save("screenshot_by_pid.bmp")
#             logger.info("截图已保存为 screenshot_by_pid.bmp")
#         else:
#             logger.error("未能获取截图")
            
#     except Exception as e:
#         logger.error(f"Main program error: {str(e)}", exc_info=True)
