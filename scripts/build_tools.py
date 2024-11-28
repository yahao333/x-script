# -*- coding: utf-8 -*-

import os
import sys
import subprocess
import shutil
import json
import re
import time
from win10toast_click import ToastNotifier

def notify_result(success, message=""):
    """Send Windows notification for build result"""
    try:
        toaster = ToastNotifier()
        title = "Build Successful" if success else "Build Failed"
        toaster.show_toast(
            title,
            message,
            duration=5,
            threaded=True,  # Don't block execution
            icon_path=None
        )
    except Exception as e:
        log(f"Failed to send notification: {e}")

def get_target_root():
    return "E:\\gitlab\\scts\\scts-backend\\tools"

# 编译target的go工程， 输出 cli.exe
def build_tools():
    app_name = "cli.exe"
    # 先切换到target_root目录
    os.chdir(get_target_root())
    target_path = os.path.join(get_target_root(), app_name)
    
    log("Starting build process...")
    try:
        # Capture output while still checking for errors
        result = subprocess.run(
            ["go", "build", "-o", app_name],
            check=True,
            capture_output=True,
            text=True
        )
        
        # Print stdout if there is any
        if result.stdout:
            log(result.stdout)
            
        log("Build completed successfully")
        notify_result(True, "Build completed successfully")
        
    except subprocess.CalledProcessError as e:
        # Print build error output
        if e.stdout:
            log(e.stdout)
        if e.stderr:
            log(e.stderr)
        notify_result(False, "Build failed. Check console for details.")
        raise Exception("Build failed") from e

# 检查环境
def check_environment():
    # 检查target_root是否存在
    if not os.path.exists(get_target_root()):
        log(f"target_root not found: {get_target_root()}")
        raise Exception(f"target_root not found: {get_target_root()}")

# 编写一个基础的print函数, 使用 sys.stdout.flush() 刷新缓冲区
def log(message):
    print(message)
    sys.stdout.flush()

def main():
    try:
        check_environment()
        build_tools()
    except Exception as e:
        log(e)

if __name__ == "__main__":
    main()

