#!/usr/bin/env python3
import sys

# MIT License

# Copyright (c) 2018-2019 The gopcua authors

# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:

# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.

# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.

from opcua import ua, Server


# https://github.com/gopcua/opcua/blob/affd2bf105fe37786d69cd3607b5f7ed085f8c90/uatest/method_server.py
def square(parent, variant):
    v = int(variant.Value)
    variant.Value = str(v * v)
    return [variant]


# https://github.com/gopcua/opcua/blob/affd2bf105fe37786d69cd3607b5f7ed085f8c90/uatest/rw_server.py
if __name__ == "__main__":
    args = sys.argv
    if len(args) != 1 and len(args) != 3:
        print("Usage: python3 opcua_server.py [path_to_server_pk.pem] [path_to_server_cert.der]")
        sys.exit(1)
    secure = len(args) == 3

    server = Server()
    server.set_endpoint("opc.tcp://0.0.0.0:48408/")
    security_policy = [
        ua.SecurityPolicyType.NoSecurity,
    ]

    if secure:
        security_policy.append(ua.SecurityPolicyType.Basic256Sha256_Sign)
        security_policy.append(ua.SecurityPolicyType.Basic256Sha256_SignAndEncrypt)
        server.load_private_key(args[1])
        server.load_certificate(args[2])

    server.set_security_policy(security_policy)

    ns = server.register_namespace("http://gopcua.com/")
    main = server.nodes.objects.add_object(ua.NodeId("main", ns), "main")
    roBool = main.add_variable(ua.NodeId("ro_bool", ns), "ro_bool", True, ua.VariantType.Boolean)
    rwBool = main.add_variable(ua.NodeId("rw_bool", ns), "rw_bool", True, ua.VariantType.Boolean)
    rwBool.set_writable()

    roInt32 = main.add_variable(ua.NodeId("ro_int32", ns), "ro_int32", 5, ua.VariantType.Int32)
    rwInt32 = main.add_variable(ua.NodeId("rw_int32", ns), "rw_int32", 5, ua.VariantType.Int32)
    rwInt32.set_writable()

    mSquare = main.add_method(ua.NodeId("square", ns), "square", square, [ua.VariantType.Int64], [ua.VariantType.Int64])

    server.start()
