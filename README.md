# anamnesis-blog

## Virtual environment
To create the virtual environment, run `python -m venv .venv` 

To activate the virtual environment, run `source .venv/bin/activate`

To exit the virtual environment, run `deactivate`

## Installing dependencies
To install the project's python dependencies, run `pip install -r requirements.txt` from within the virtual environment.
To install Piper-TTS model, run `python3 -m piper.download_voices en_US-amy-medium`

## Running the project
From within the virtual environment, run `fastapi dev main.py`
