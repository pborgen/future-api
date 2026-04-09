# Appointment Scheduling

**BACKEND ENGINEERING Project — future.co**

## Overview

Build an HTTP JSON API in Go that allows clients to schedule video call appointments with their trainers (coaches).

## Motivation

At Future, our trainers often schedule video calls with clients to check in on progress and make adjustments. We'd like an API to allow a client to schedule a video call with their trainer.

## Business Rules

- The client should be able to pick from a list of available times.
- Appointments for a coach **must not overlap**.
- All appointments are **30 minutes long**.
- Appointments must be scheduled at **:00 or :30** minutes after the hour.
- **Business hours: Monday–Friday, 8am–5pm Pacific Time.**

## API Endpoints

### 1. Get available appointment times for a trainer

Get a list of available appointment times for a trainer between two dates.

**Parameters:**
- `trainer_id`
- `starts_at`
- `ends_at`

**Returns:** list of available appointment times

### 2. Create a new appointment

Post a new appointment (as JSON).

**Fields:**
- `trainer_id`
- `user_id`
- `starts_at`
- `ends_at`

### 3. Get scheduled appointments for a trainer

Get a list of scheduled appointments for a trainer.

**Parameters:**
- `trainer_id`

## Data Format

The included file `appointments.json` contains the current list of appointments in this format:

```json
[
  {
    "id": 1,
    "trainer_id": 1,
    "user_id": 2,
    "starts_at": "2019-01-25T09:00:00-08:00",
    "ends_at": "2019-01-25T09:30:00-08:00"
  }
]
```

You can store appointments in this file, a database, or any back end storage you prefer.

## Storage Requirements

- Implement a simple database storage layer.
- **Postgres is preferred.**
- Provide a **Dockerfile** (or equivalent) so the reviewer can follow setup steps.
