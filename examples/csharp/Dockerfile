FROM mcr.microsoft.com/dotnet/sdk:8.0 AS build
WORKDIR /src
COPY csharp.csproj .
RUN dotnet restore
COPY . .
RUN dotnet publish -c Release -o /app/publish

FROM mcr.microsoft.com/dotnet/aspnet:8.0 AS runtime
WORKDIR /app
COPY --from=build /app/publish .
EXPOSE 8083
ENV ASPNETCORE_URLS=http://0.0.0.0:8083
ENV OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318
ENV OTEL_SERVICE_NAME=examples-csharp
ENTRYPOINT ["dotnet", "csharp.dll"]


