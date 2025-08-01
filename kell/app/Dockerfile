# use an official Node.js runtime as a parent image
FROM node:20-slim

# set the working dir in the container
WORKDIR /app

# copy package.json and package-lock.json to the working dir
COPY package*.json ./

# install any needed packages
RUN npm install

# bundle app source
COPY . .

# make port 8080 available to the world outside this container
EXPOSE 8080

# define the command to run the app
CMD ["node", "server.js"]