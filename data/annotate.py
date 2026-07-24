#!/usr/bin/env python3

import argparse
import re


def get_field_info(definition: int):
    field_type = definition & 0xF
    shift = (definition >> 4) & 0xF
    param = definition >> 8

    if field_type == 0:
        return "AgentId", "type=0 size=4"

    elif field_type == 1:
        return "Float32", "type=1 size=4"

    elif field_type == 2:
        return "Vec2", "type=2 size=8"

    elif field_type == 3:
        return "Vec3", "type=3 size=12"

    elif field_type in (4, 8):
        names = {
            1: "Uint8",
            2: "Uint16",
            4: "Uint32",
            8: "Uint64",
        }
        return names.get(param, f"Integer{param * 8}"), f"type={field_type} size={param}"

    elif field_type in (5, 9):
        return "FixedBytes", f"type={field_type} bytes={param}"

    elif field_type in (6, 10):
        return "Ignore", f"type={field_type}"

    elif field_type == 7:
        return "String16", f"type=7 maxChars={param}"

    elif field_type == 11:
        size = 1 << shift
        return f"Array{size * 8}", f"type=11 lengthBytes={size} maxCount={param}"

    elif field_type == 12:
        return "Nested", "type=12"

    raise ValueError(f"Unhandled field type {field_type}")


HEX_RE = re.compile(r'^(\s*)(0x[0-9A-Fa-f]+)(,?)(.*)$')


def annotate_line(line: str, verbose: bool = False) -> str:
    m = HEX_RE.match(line)
    if not m:
        return line

    indent, hex_value, comma, rest = m.groups()

    # Skip map keys like:
    # 0x0001: {
    if ":" in rest:
        return line

    value = int(hex_value, 16)

    try:
        field_name, info = get_field_info(value)
    except Exception as e:
        field_name = str(e)
        info = ""

    if verbose:
        comment = f"{field_name} ({info})"
    else:
        comment = field_name

    # Replace any existing trailing comment.
    rest = re.sub(r'//.*$', '', rest).rstrip()

    if rest:
        return f"{indent}{hex_value}{comma}{rest} // {comment}\n"
    else:
        return f"{indent}{hex_value}{comma} // {comment}\n"


def annotate_file(input_file: str, output_file: str, verbose: bool):
    with open(input_file, "r", encoding="utf-8") as f:
        lines = f.readlines()

    annotated = [annotate_line(line, verbose) for line in lines]

    with open(output_file, "w", encoding="utf-8") as f:
        f.writelines(annotated)


def main():
    parser = argparse.ArgumentParser(
        description="Annotate Go field definition files with decoded field types."
    )
    parser.add_argument("input", help="Input Go file")
    parser.add_argument(
        "-o",
        "--output",
        help="Output file (default: overwrite input)"
    )
    parser.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        help="Include decoded descriptor information in comments"
    )

    args = parser.parse_args()

    output = args.output or args.input
    annotate_file(args.input, output, args.verbose)


if __name__ == "__main__":
    main()
