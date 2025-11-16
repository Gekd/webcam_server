# Garage48 15-Year Anniversary Hackathon Project Wektor Android phones as camera nodes server

To run this project:
<br> 1) Create a Python virtual env
<br>```python3 -m venv server```
<br> 2) Open virtual env
<br>```source server/bin/activate```
<br> 3) Download packages
<br>```pip install -r requirements.txt```
<br> 4) Run Go ...
<br>```go mod tidy```
<br> 5) Run Python script
<br>```python detector_server.py```
<br> 6) Run Go server
<br>``` go run main.go```
<br> 7) Open dashboard
<br>```http://127.0.0.1:8080/```
