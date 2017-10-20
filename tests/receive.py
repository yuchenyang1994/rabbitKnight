#!/usr/bin/env python
# -*- coding: utf-8 -*-


from flask import Flask
from flask import request
app = Flask(__name__)

@app.route('/test')
def hello_world():
    if request.method == "POST":
        print(request.data)

if __name__ == '__main__':
    app.run()
