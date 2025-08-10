require 'sinatra'

set :bind, '0.0.0.0'
set :port, 4567

get '/' do
  'Hello from Ruby sample!'
end

get '/work' do
  # Simulate some work
  sum = 0
  1000.times { |i| sum += i }
  "Work done: #{sum}"
end


