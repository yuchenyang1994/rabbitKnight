#!/usr/bin/env python
# -*- coding: utf-8 -*-


from flask import Flask
from flask import request
app = Flask(__name__)

@app.route('/test', methods=['GET','POST'])
def hello_world():
    if request.method == "POST":
        print(request.data)
        return "hello, word"

if __name__ == '__main__':
    app.run()
