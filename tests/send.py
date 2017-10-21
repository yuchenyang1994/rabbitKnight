#!/usr/bin/env python
# -*- coding: utf-8 -*-

import pika
import json

msg = json.dumps({"message":"hello,word"})
parameters = pika.URLParameters('amqp://guest:guest@172.17.0.2:5672')
connection = pika.BlockingConnection(parameters)
channel = connection.channel()
for i in range(10000):
    channel.basic_publish(
        'test_exchange',
        'test',
        msg)
channel.close()

