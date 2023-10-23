import sys
import os

def main():
    print("stdout line")
    sys.stderr.write("error line\n")
    
    argument = ""

    if len(sys.argv) > 1:
        argument = sys.argv[1]

    prefix = os.environ.get("PREFIX", "")
    print(f"{prefix}-{argument}")

if __name__ == "__main__":
    main()
