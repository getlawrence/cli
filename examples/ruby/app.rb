require 'sinatra'

get '/' do
  'Hello from Ruby sample!'
end

get '/work' do
  # Simulate some work
  sum = 0
  1000.times { |i| sum += i }
  "Work done: #{sum}"
end


