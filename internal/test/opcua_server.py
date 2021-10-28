#!/usr/bin/env python3

from opcua import ua, Server

# Code from https://github.com/gopcua/opcua/blob/affd2bf105fe37786d69cd3607b5f7ed085f8c90/uatest/method_server.py
def square(parent, variant):
    v = int(variant.Value)
    variant.Value = str(v * v)
    return [variant]

# Code from https://github.com/gopcua/opcua/blob/affd2bf105fe37786d69cd3607b5f7ed085f8c90/uatest/rw_server.py
if __name__ == "__main__":
    server = Server()
    server.set_endpoint("opc.tcp://0.0.0.0:48408/")

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