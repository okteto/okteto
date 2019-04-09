import React, { Component } from 'react';
import { ToastContainer, toast, cssTransition } from 'react-toastify';

import Icon from 'components/Icon';

import 'react-toastify/dist/ReactToastify.css';
import 'components/Notification.scss';

let lastActiveToast = null;
const defaultAutoCloseTime = 5000;
const defaultToastTransition = cssTransition({
  enter: 'toastIn',
  exit: 'toastOut',
  duration: 800
});

class Notification extends Component {
  constructor(props) {
    super(props);
  }

  render() {
    return (
      <div className='Notification'>
        <ToastContainer 
          className="toast-container"
          toastClassName="toast"
          bodyClassName="toast-body"
          progressClassName="toast-progress"
          position="top-center"
          autoClose={defaultAutoCloseTime}
          hideProgressBar
          newestOnTop 
        />
      </div>
    );
  }
}

export const notify = (message, type = 'info') => {
  let notifyFunc;
  switch (type) {
    case 'warning':
      notifyFunc = toast.warning; 
      break;
    case 'error': 
      notifyFunc = toast.error; 
      break;
    case 'info':
    default:
      notifyFunc = toast;
      break;
  }

  if (lastActiveToast && lastActiveToast.message === message) {
    // Avoid duplication and extend time of duplicated message.
    toast.update(lastActiveToast.id, { 
      autoClose: defaultAutoCloseTime
    });
  } else {
    // Create a new toast.
    lastActiveToast = {
      id: notifyFunc(message, {
        onClose: function() {
          // Here 'this' is the toast object.
          if (lastActiveToast && lastActiveToast.id === this.id) {
            lastActiveToast = null;
          }
        },
        className: `toast toast-type-${type}`,
        closeButton: <Icon className="close-button" icon="close" color="#fff" />,
        transition: defaultToastTransition
      }),
      message
    };
  }
};

export default Notification;
