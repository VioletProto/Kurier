"""Only network escape for isolated children: a Unix socket to the meter."""

import asyncio
import os
import resource
import sys


async def splice(reader, writer):
    try:
        while data := await reader.read(65536):
            writer.write(data)
            await writer.drain()
    finally:
        writer.close()


async def main():
    mode, socket, port, *command = sys.argv[1:]
    resource.setrlimit(resource.RLIMIT_CORE, (0, 0))

    async def relay(reader, writer):
        try:
            other_r, other_w = (
                await asyncio.open_connection("127.0.0.1", int(port))
                if mode == "host"
                else await asyncio.open_unix_connection(socket)
            )
            await asyncio.gather(
                splice(reader, other_w), splice(other_r, writer), return_exceptions=True
            )
        except OSError:
            writer.close()

    if mode == "host":
        server = await asyncio.start_unix_server(relay, socket)
        os.chmod(socket, 0o600)
        async with server:
            await server.serve_forever()
    elif mode == "inside":
        server = await asyncio.start_server(relay, "127.0.0.1", int(port))
        async with server:
            child = await asyncio.create_subprocess_exec(*command)
            return await child.wait()
    else:
        raise ValueError("Choose host or inside")


if __name__ == "__main__":
    sys.exit(asyncio.run(main()) or 0)
