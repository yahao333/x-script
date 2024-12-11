# -*- coding: utf-8 -*-
import pygetwindow as gw
import pyautogui
from PIL import Image
import asyncio
import winrt.windows.media.ocr as ocr
import io

import winreg
import sys

from ctypes import windll, create_string_buffer, byref, c_ulong

from winrt.windows.devices.bluetooth.advertisement import BluetoothLEAdvertisementWatcher

def get_wx_version():
    """获取微信版本号"""

    try:
        with winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Software\Tencent\WXWork", 0, winreg.KEY_READ) as key:
            return winreg.QueryValueEx(key, "Version")[0]
    except Exception as e:
        print("打开注册表失败：{}".format(e))
        return None
    
# 编写一个基础的print函数, 使用 sys.stdout.flush() 刷新缓冲区
def log(message):
    print(message)
    sys.stdout.flush()

def capture_wxwork_screenshot():
    # 获取微信窗口
    wxwork_window = gw.getWindowsWithTitle('WeChat')[0]
    if not wxwork_window:
        print("未找到微信窗口")
        return None

    # Capture the screenshot of the window
    screenshot = pyautogui.screenshot(region=(wxwork_window.left, wxwork_window.top, wxwork_window.width, wxwork_window.height))
    screenshot.save('tmp.png')
    return screenshot

async def perform_ocr_on_screenshot(image):
    # Crop the image to the desired region
    cropped_image = image.crop((0, 0, 420, 320))
    # Perform OCR
    text = await get_text_from_image(cropped_image)
    return text

async def get_text_from_image(image):
    pass
    # 将 PIL Image 转换为字节流
    # img_byte_arr = io.BytesIO()
    # image.save(img_byte_arr, format='PNG')
    # img_byte_arr = img_byte_arr.getvalue()

    # # 创建 InMemoryRandomAccessStream
    # stream = streams.InMemoryRandomAccessStream()
    # writer = streams.DataWriter(stream)
    # writer.write_bytes(img_byte_arr)
    # await writer.store_async()
    # stream.seek(0)

    # # 创建 SoftwareBitmap
    # decoder = await imaging.BitmapDecoder.create_async(stream)
    # software_bitmap = await decoder.get_software_bitmap_async()

    # # 创建 OCR 引擎
    # ocr_engine = ocr.OcrEngine.try_create_from_language(ocr.OcrEngine.available_recognition_languages[0])
    # ocr_result = await ocr_engine.recognize_async(software_bitmap)

    # # 提取识别的文本
    # text = "\n".join([line.text for line in ocr_result.lines])
    # return text

def windows_ocr(image_path):
    # 加载图像
    pil_image = Image.open(image_path)
    
    # 获取系统语言
    language = ocr.OcrEngine.get_available_recognizer_languages()[0]
    engine = ocr.OcrEngine.try_create_from_language(language)
    
    # 转换图像
    bitmap = pil_image.convert('RGB')
    
    # 执行OCR
    result = engine.recognize_async(bitmap).get()
    return result.text

async def async_windows_ocr(image_path):
    # 加载图像
    pil_image = Image.open(image_path)
    
    # 获取系统语言
    language = ocr.OcrEngine.get_available_recognizer_languages()[0]
    engine = ocr.OcrEngine.try_create_from_language(language)
    
    # 转换图像
    bitmap = pil_image.convert('RGB')
    
    # 执行OCR (使用异步调用)
    result = await engine.recognize_async(bitmap)
    return result.text

async def scan():
    watcher = BluetoothLEAdvertisementWatcher()

    event_loop = asyncio.get_running_loop()
    received_queue = asyncio.Queue()

    # this event is expected zero or more times, so we use a queue to pipe the results
    def handle_received(sender, event_args):
      received_queue.put_nowait(event_args)

    stopped_future = event_loop.create_future()

    # this event is expected *exactly* once, so we use a future to capture the result
    def handle_stopped(sender, event_args):
      stopped_future.set_result(event_args)

    received_token = watcher.add_received(
        lambda s, e: event_loop.call_soon_threadsafe(handle_received, s, e)
    )
    stopped_token = watcher.add_stopped(
        lambda s, e: event_loop.call_soon_threadsafe(handle_stopped, s, e)
    )

    try:
        print("scanning...")
        watcher.start()

        # this is the consumer for the received event queue
        async def print_received():
          while True:
            event_args = await received_queue.get()
            print(
                "received:",
                event_args.bluetooth_address.to_bytes(6, "big").hex(":"),
                event_args.raw_signal_strength_in_d_bm, "dBm",
            )

        printer_task = asyncio.create_task(print_received())

        # since the print task is an infinite loop, we have to cancel it when we don't need it anymore
        stopped_future.add_done_callback(printer_task.cancel)

        # scan for 10 seconds or until an unexpected stopped event (due to error)
        done, pending = await asyncio.wait(
            [stopped_future, printer_task], timeout=10, return_when=asyncio.FIRST_COMPLETED
        )

        if stopped_future in done:
            print("unexpected stopped event", stopped_future.result().error)
        else:
            print("stopping...")
            watcher.stop()
            await stopped_future
    finally:
        # event handler are removed in a finally block to ensure we don't leak
        watcher.remove_received(received_token)
        watcher.remove_stopped(stopped_token)

    print("done.")

async def test1():
    screenshot = capture_wxwork_screenshot()
    if screenshot:
        ocr_result = await async_windows_ocr('test.png')
        print("OCR Result:")
        print(ocr_result)

async def test2():
    ocr_result = await async_windows_ocr('test.png')
    print("OCR Result:")
    print(ocr_result)

def test3():
    # 调用kernel32.dll
    kernel32 = windll.kernel32    
    # 获取系统信息
    buffer = create_string_buffer(200)
    kernel32.GetComputerNameA(buffer, byref(c_ulong(200)))
    computer_name = buffer.value
    print(computer_name)

def test4():
    asyncio.run(scan())

def main():
    # wx_version = get_wx_version()
    # log(wx_version)
    # asyncio.run(test1())
    test4()
    
if __name__ == "__main__":
    main()
