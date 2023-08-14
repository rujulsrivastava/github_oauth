[![Review Assignment Due Date](https://classroom.github.com/assets/deadline-readme-button-24ddc0f5d75046c5622901739e7c5dd533143b0c8e959d652212380cedb1ea36.svg)](https://classroom.github.com/a/e3nG7TEg)
[![Open in Visual Studio Code](https://classroom.github.com/assets/open-in-vscode-c66648af7eb3fe8bc4f294546bfd86ef473780cde1dea487d3c4ff354943c9ae.svg)](https://classroom.github.com/online_ide?assignment_repo_id=10773666&assignment_repo_type=AssignmentRepo)
# Github API Repository Fetcher

This project is a web application that fetches data from the Github API using OAuth authentication and stores it in a Postgres database. The data is normalized and deduplicated before being stored. The application fetches repository data dynamically along with its owner information and can handle both public and private repositories of a user. The fetched data is then mapped to a CSV format, and the user can download this CSV by hitting an endpoint.

## Features

*   OAuth authentication with the Github API
*   Fetch and store repository data along with owner information
*   Support for public and private repositories
*   Proper error handling and retries in case of network failures or other issues
*   Deduplication of fetched data using Repo ID and Owner ID
*   CSV generation and download endpoint

## Workflow Diagram
<img width="251" alt="image" src="https://user-images.githubusercontent.com/71875214/230892718-6f71bc4d-28ee-4ca9-9996-c04e00798574.png">

## Installation and Usage

### Prerequisites

*   Go (1.15 or later)
*   PostgreSQL (9.6 or later)
*   A GitHub account with a registered OAuth App (for client ID and secret) - follow instructions at https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/creating-an-oauth-app

### OAuth

1. Generate OAuth app with your preferred name.
2. Provide homepage URL as http://localhost:8080
3. Provide callback URL as http://localhost:8080/callback

### Setup

1.  Clone this repository to your local machine.
    
    ```bash
    git clone https://github.com/rujulsrivastava/github_oauth.git 
    cd <project-directory>
    cd master
    ```
    
2.  Copy the `.env.sample` file to a new file called `.env` and fill in the required variables:
    
    ```bash
    cp .env.sample .env
    ```
    
    Fill in the following variables in the `.env` file:
    
    ```makefile
    GITHUB_CLIENT_ID=your_client_id GITHUB_CLIENT_SECRET=your_client_secret DB_USERNAME=your_database_username DB_PASSWORD=your_database_password DB_NAME=your_database_name
    ```
    
3.  Ensure you have the Go language installed on your machine and run the following command to build the project:
    
    ```go
    go build
    ```
    
4.  Run the compiled binary:
    
    ```bash
    ./master.exe
    ```
    
5.  Open your browser and visit [`http://localhost:8080`](http://localhost:8080) to access the application.
    
6.  Click the "Authorize" button to authenticate with your Github account and fetch your repositories.
    <img width="959" alt="image" src="https://user-images.githubusercontent.com/71875214/230892837-017cc4d0-6720-467d-97fe-095cd2412def.png">

7.  After the repositories have been fetched and saved, click the "Download CSV" button to download the generated CSV file.
    <img width="960" alt="image" src="https://user-images.githubusercontent.com/71875214/230892792-effc7b90-d00b-4232-9888-c967fafb6a50.png">

    

## CSV Fields

The CSV file contains the following fields:

*   Owner ID
*   Owner Name
*   Owner Email (Empty if null)
*   Repo ID
*   Repo Name
*   Status (Public or Private)
*   Stars Count

## Project Structure

*   `main.go`: Contains the main application logic, including HTTP handlers, OAuth flow, and database operations.
*   `logger.go`: Sets up the logger for the application.
*   `.env`: Contains environment variables required for the application, such as Github API credentials and database connection details.
*   `index.html`: The main HTML file for the web application.
*   `error.log`: Contains logs of errors encountered to assist in easy debugging and error handling

## Known Issues

Emails are being fetched as evident in error.log (depending on their public/private status) but not being aptly translated into the database.
<img width="701" alt="image" src="https://user-images.githubusercontent.com/71875214/230901911-93ffbbc9-0cde-4c97-be1b-6e2727deffca.png">


Dockerization of the project is in process. Current updates can be viewed in the branch `Docker`.
It will continue to be in process regardless of the outcome. We love to build here!

