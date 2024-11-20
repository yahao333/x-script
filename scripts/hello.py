# -*- coding: utf-8 -*-

import time
import sys
def main():
    for i in range(5):
        print(f"Hello, World! {i}")
        sys.stdout.flush()
        time.sleep(1)

if __name__ == "__main__":
    main()
